package mitm

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/cocodedk/parvaz/core/protocol"
)

// Relayer is what the Interceptor needs from the relay package: one call
// per intercepted HTTP request. Kept as an interface here so the mitm
// package has no direct dependency on core/relay, and so tests can stub.
type Relayer interface {
	Do(ctx context.Context, req protocol.Request) (*protocol.Response, error)
}

// Interceptor terminates browser TLS on a rawConn (typically a SOCKS5
// CONNECT stream post-reply), parses the plaintext HTTP requests, hands
// them to Relayer.Do, and writes responses back to the browser.
type Interceptor struct {
	CA     *CA
	Relay  Relayer
	Logger *slog.Logger

	// HandshakeTimeout bounds the initial TLS handshake. Zero = default 15s.
	HandshakeTimeout time.Duration

	// IdleTimeout bounds how long the keep-alive loop waits between requests.
	// Without this, a browser that walks away from a connection would pin a
	// goroutine on tlsConn.Read forever. Zero = default 120s.
	IdleTimeout time.Duration

	// leafCache memoizes leaf certificates by host. A single mitm process
	// sees the same host many times (page + assets); re-signing per hit is
	// wasteful.
	leafMu    sync.Mutex
	leafCache map[string]*tls.Certificate
}

const (
	defaultInterceptHandshakeTimeout = 15 * time.Second
	defaultInterceptIdleTimeout      = 120 * time.Second
)

func (i *Interceptor) logger() *slog.Logger {
	if i.Logger != nil {
		return i.Logger
	}
	return slog.Default()
}

// Intercept owns rawConn for the lifetime of the call; on return it's
// closed. nil error == clean client EOF. host may be an IP literal
// (tun2socks) or hostname (SOCKS5 CONNECT); GetCertificate mints a
// leaf for the SNI the browser sends, and post-handshake ServerName
// drives the upstream URL.
func (i *Interceptor) Intercept(ctx context.Context, rawConn net.Conn, host string, port uint16) error {
	defer rawConn.Close()

	tlsConn := tls.Server(rawConn, i.tlsConfig(host))

	hsTimeout := i.HandshakeTimeout
	if hsTimeout == 0 {
		hsTimeout = defaultInterceptHandshakeTimeout
	}
	_ = rawConn.SetDeadline(time.Now().Add(hsTimeout))
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return fmt.Errorf("mitm: tls handshake: %w", err)
	}
	effectiveHost := host
	if sni := tlsConn.ConnectionState().ServerName; sni != "" {
		effectiveHost = sni
	}
	idleTimeout := i.IdleTimeout
	if idleTimeout == 0 {
		idleTimeout = defaultInterceptIdleTimeout
	}

	br := bufio.NewReader(tlsConn)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		_ = rawConn.SetReadDeadline(time.Now().Add(idleTimeout))
		req, err := http.ReadRequest(br)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return nil
			}
			return fmt.Errorf("mitm: read request: %w", err)
		}
		_ = rawConn.SetDeadline(time.Time{}) // response + body may be large
		if err := i.roundTrip(ctx, tlsConn, req, effectiveHost, port); err != nil {
			return err
		}
		if req.Close {
			return nil
		}
	}
}

func (i *Interceptor) roundTrip(ctx context.Context, w io.Writer, req *http.Request, host string, port uint16) error {
	reqURL, err := buildTargetURL(host, port, req)
	if err != nil {
		return fmt.Errorf("mitm: build URL: %w", err)
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return fmt.Errorf("mitm: read request body: %w", err)
	}
	_ = req.Body.Close()

	protoReq := protocol.Request{
		Method:      req.Method,
		URL:         reqURL,
		Header:      req.Header.Clone(),
		Body:        body,
		ContentType: req.Header.Get("Content-Type"),
		// Browser must see 3xx itself — auto-follow at the relay would
		// hide the URL change from the address bar.
		FollowRedirects: false,
	}
	resp, err := i.Relay.Do(ctx, protoReq)
	if err != nil {
		i.logger().Debug("mitm: relay failed", "host", host, "url", reqURL, "err", err)
		return writeBadGateway(w, err.Error())
	}
	return writeResponse(w, req, resp)
}

// buildTargetURL reconstructs an https:// URL from the host+port the
// browser originally CONNECTed to, plus the path/query from the request
// line (which is path-only on HTTPS requests).
func buildTargetURL(host string, port uint16, req *http.Request) (string, error) {
	u := &url.URL{Scheme: "https", Path: "/"}
	if port == 443 {
		u.Host = host
	} else {
		u.Host = net.JoinHostPort(host, strconv.Itoa(int(port)))
	}
	if req.URL != nil {
		u.Path = req.URL.Path
		u.RawPath = req.URL.RawPath
		u.RawQuery = req.URL.RawQuery
	}
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String(), nil
}

func (i *Interceptor) tlsConfig(fallbackHost string) *tls.Config {
	return &tls.Config{
		// Mint the leaf certificate for whatever SNI the browser sent.
		// If SNI is empty (bare-IP target, e.g. tun2socks surfacing a
		// DNS-less IP literal), fall back to the host we got from the
		// outer protocol — the browser's validation will still flag
		// the mismatch, but at least we get a usable cert.
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			name := hello.ServerName
			if name == "" {
				name = fallbackHost
			}
			return i.leafFor(name)
		},
		MinVersion: tls.VersionTLS12,
		// Chrome/Firefox ALPN-advertise h2 first; without this we'd negotiate
		// HTTP/2 and then fail on http.ReadRequest (which only speaks h1).
		NextProtos: []string{"http/1.1"},
	}
}

func (i *Interceptor) leafFor(host string) (*tls.Certificate, error) {
	i.leafMu.Lock()
	defer i.leafMu.Unlock()
	if cert, ok := i.leafCache[host]; ok {
		return cert, nil
	}
	cert, err := i.CA.Sign(host)
	if err != nil {
		return nil, err
	}
	if i.leafCache == nil {
		i.leafCache = map[string]*tls.Certificate{}
	}
	i.leafCache[host] = cert
	return cert, nil
}
