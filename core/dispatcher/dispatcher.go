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

// DefaultAllowList holds the subset of the mhrv-rs list that is safe for
// Path 1 today. YouTube / ytimg / ggpht are intentionally omitted: they
// are DPI-blocked in our target region, and socks5 has no direct→MITM
// fallback (a failed Dial returns replyFail to the client immediately).
// The SNI-rewrite path lands them in a follow-up PR; until then they
// fall through to MITM + Apps Script relay, which is slow but works.
var DefaultAllowList = []string{
	"*.google.com",
	"*.googleusercontent.com",
	"*.gstatic.com",
	"*.googleapis.com",
}

// Dispatcher routes each SOCKS5 CONNECT to either direct TCP (Path 1)
// or MITM + relay (Path 3) based on AllowList.
type Dispatcher struct {
	// AllowList holds hostname patterns for the direct-TCP path. Exact
	// match, or a leading "*." wildcard: "*.google.com" matches
	// "google.com", "www.google.com", and "a.b.google.com".
	AllowList []string

	// Interceptor handles the MITM path. Must be non-nil.
	Interceptor Interceptor

	// DialContext opens the TCP connection for the direct path. If nil,
	// uses (&net.Dialer{Timeout: 10s}).DialContext.
	DialContext func(ctx context.Context, network, address string) (net.Conn, error)

	// Logger — nil uses slog.Default().
	Logger *slog.Logger
}

const defaultDialTimeout = 10 * time.Second

// Dial implements socks5.Dialer.
func (d *Dispatcher) Dial(ctx context.Context, host string, port uint16) (net.Conn, error) {
	if d.matchesAllowList(host) {
		d.logger().Debug("dispatcher: routing",
			"host", host, "port", port, "path", "direct")
		return d.dialDirect(ctx, host, port)
	}
	d.logger().Debug("dispatcher: routing",
		"host", host, "port", port, "path", "mitm")
	return d.dialMITM(ctx, host, port)
}

// matchesAllowList reports whether host matches any AllowList pattern.
// Case-insensitive on both sides.
func (d *Dispatcher) matchesAllowList(host string) bool {
	h := strings.ToLower(host)
	for _, pattern := range d.AllowList {
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

func (d *Dispatcher) logger() *slog.Logger {
	if d.Logger != nil {
		return d.Logger
	}
	return slog.Default()
}
