// Package dispatcher decides how to serve each SOCKS5 CONNECT: pure TCP
// proxy for Google-owned hosts that are not DPI-blocked (Path 1 — "direct
// tunnel"), or MITM + Apps Script relay for everything else (Path 3 —
// "MITM + relay"). A third path for SNI-rewrite of Google-owned but
// DPI-blocked domains (YouTube, etc.) is planned as a follow-up PR.
//
// The dispatcher satisfies socks5.Dialer structurally, so it plugs into
// the existing SOCKS5 server without modifying that package.
package dispatcher

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"
)

// Interceptor is what the dispatcher needs from mitm.Interceptor —
// kept as an interface so core/dispatcher doesn't import core/mitm here.
// *mitm.Interceptor satisfies this structurally.
type Interceptor interface {
	Intercept(ctx context.Context, rawConn net.Conn, host string, port uint16) error
}

// SNITunneler is what the dispatcher needs from mitm.SNITunnel — the
// third path that rescues DPI-blocked Google-owned hosts by MITMing the
// browser and re-encrypting upstream with a safe SNI. *mitm.SNITunnel
// satisfies this structurally.
type SNITunneler interface {
	Tunnel(ctx context.Context, rawConn net.Conn, host string, port uint16) error
}

// DefaultAllowList holds the subset of mhrv-rs's set that is safe for
// Path 1 today. These are Google-owned hostnames that are NOT DPI-blocked
// in our target region, so the browser can reach them directly without
// any SNI masking.
var DefaultAllowList = []string{
	"*.google.com",
	"*.googleusercontent.com",
	"*.gstatic.com",
	"*.googleapis.com",
}

// DefaultSNIRewriteList holds the Google-owned hostnames that ARE
// DPI-blocked in our target region and need SNI masking to work. The
// dispatcher terminates browser TLS locally, opens an upstream TLS
// connection to a Google edge IP with SNI rewritten to a safe value
// (typically www.google.com), and Google's edge routes internally by
// the Host header. Skips Apps Script entirely — critical for video.
var DefaultSNIRewriteList = []string{
	"*.youtube.com",
	"*.ytimg.com",
	"*.ggpht.com",
}

// Dispatcher routes each SOCKS5 CONNECT to one of three paths based on
// the target host. Wildcard matching rules for AllowList and
// SNIRewriteList: exact match, or leading "*." — "*.google.com" matches
// "google.com", "www.google.com", and "a.b.google.com".
type Dispatcher struct {
	// AllowList is Path 1 (direct TCP): browser talks end-to-end TLS to
	// the real target. Zero Apps Script quota.
	AllowList []string

	// SNIRewriteList is Path 2: MITM the browser, open upstream TLS via
	// SNITunnel with a safe SNI, pipe plaintext. Skips Apps Script.
	SNIRewriteList []string

	// Interceptor handles Path 3 (MITM + Apps Script relay) — the
	// catch-all for everything that isn't in AllowList or SNIRewriteList.
	// Must be non-nil.
	Interceptor Interceptor

	// SNITunnel executes Path 2. If nil, SNIRewriteList entries fall
	// through to Path 3 (safer default than failing closed).
	SNITunnel SNITunneler

	// DNSTCP is the TCP/53 shim for CONNECTs that exactly target
	// DNSHost:DNSPort. Wired alongside the UDP path so resolver TCP
	// fallback for the synthetic in-TUN DNS address reaches the same
	// DoH backend. Nil disables the route — those CONNECTs fall
	// through to MITM (probably wrong, but matches pre-M15b-beta).
	DNSTCP  DNSTCPHandler
	DNSHost string // e.g. "10.0.0.2"; matched case-insensitively
	DNSPort uint16 // e.g. 53

	// DialContext opens the TCP connection for the direct path. If nil,
	// uses (&net.Dialer{Timeout: 10s}).DialContext.
	DialContext func(ctx context.Context, network, address string) (net.Conn, error)

	// Logger — nil uses slog.Default().
	Logger *slog.Logger
}

const defaultDialTimeout = 10 * time.Second

// Dial implements socks5.Dialer.
func (d *Dispatcher) Dial(ctx context.Context, host string, port uint16) (net.Conn, error) {
	switch {
	case d.isDNSTCPTarget(host, port):
		d.logger().Debug("dispatcher: routing",
			"host", host, "port", port, "path", "dns-tcp")
		return d.dialDNSTCP(ctx)
	case matchesPatternList(host, d.AllowList):
		d.logger().Debug("dispatcher: routing",
			"host", host, "port", port, "path", "direct")
		return d.dialDirect(ctx, host, port)
	case matchesPatternList(host, d.SNIRewriteList):
		// Misconfig check: a non-empty SNIRewriteList with a nil SNITunnel
		// is almost always a wiring mistake — silently falling back to
		// MITM+relay would burn Apps Script quota on every YouTube hit
		// with no signal to the operator. Fail loudly instead.
		if d.SNITunnel == nil {
			return nil, errors.New(
				"dispatcher: host matches SNIRewriteList but SNITunnel is nil (misconfig)")
		}
		d.logger().Debug("dispatcher: routing",
			"host", host, "port", port, "path", "sni-rewrite")
		return d.dialViaSNITunnel(ctx, host, port)
	default:
		d.logger().Debug("dispatcher: routing",
			"host", host, "port", port, "path", "mitm")
		return d.dialMITM(ctx, host, port)
	}
}

// matchesAllowList is retained for test readability; the underlying
// matcher is reused for SNIRewriteList too.
func (d *Dispatcher) matchesAllowList(host string) bool {
	return matchesPatternList(host, d.AllowList)
}

// matchesPatternList — case-insensitive, exact-match or leading "*."
// wildcard suffix. Embedded substrings do not match.
func matchesPatternList(host string, patterns []string) bool {
	h := strings.ToLower(host)
	for _, pattern := range patterns {
		p := strings.ToLower(pattern)
		if strings.HasPrefix(p, "*.") {
			suffix := p[2:]
			if h == suffix || strings.HasSuffix(h, "."+suffix) {
				return true
			}
		} else if h == p {
			return true
		}
	}
	return false
}

func (d *Dispatcher) dialDirect(ctx context.Context, host string, port uint16) (net.Conn, error) {
	dial := d.DialContext
	if dial == nil {
		dialer := &net.Dialer{Timeout: defaultDialTimeout}
		dial = dialer.DialContext
	}
	return dial(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(int(port))))
}

// dialMITM spawns Interceptor.Intercept on the server side of a net.Pipe
// and returns the client side to the caller. SOCKS5's io.Copy loop then
// bridges the real client conn ↔ pipeClientSide, which the interceptor
// sees on pipeServerSide as its "raw browser conn".
func (d *Dispatcher) dialMITM(ctx context.Context, host string, port uint16) (net.Conn, error) {
	if d.Interceptor == nil {
		return nil, errors.New("dispatcher: Interceptor not configured")
	}
	serverSide, clientSide := net.Pipe()
	go func() {
		if err := d.Interceptor.Intercept(ctx, serverSide, host, port); err != nil {
			d.logger().Debug("dispatcher: interceptor ended",
				"host", host, "port", port, "err", err)
		}
	}()
	return clientSide, nil
}

// dialViaSNITunnel is Path 2 — same pipe trick as dialMITM but the
// goroutine runs SNITunnel.Tunnel instead of Interceptor.Intercept.
func (d *Dispatcher) dialViaSNITunnel(ctx context.Context, host string, port uint16) (net.Conn, error) {
	serverSide, clientSide := net.Pipe()
	go func() {
		if err := d.SNITunnel.Tunnel(ctx, serverSide, host, port); err != nil {
			d.logger().Debug("dispatcher: SNI tunnel ended",
				"host", host, "port", port, "err", err)
		}
	}()
	return clientSide, nil
}

func (d *Dispatcher) logger() *slog.Logger {
	if d.Logger != nil {
		return d.Logger
	}
	return slog.Default()
}
