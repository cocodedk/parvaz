package mitm

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"time"
)

// SNITunnel implements the "SNI-rewrite" path: terminate the browser's
// TLS locally using a leaf signed by the CA, open an upstream TLS
// connection via UpstreamDial (which is expected to present a
// DPI-friendly SNI like www.google.com), then pipe plaintext HTTP bytes
// between the two TLS sessions. Google's edge sees SNI=www.google.com
// but routes internally by the Host header in the plaintext request.
//
// No HTTP parsing, no Apps Script quota — just byte-for-byte proxying
// after both TLS layers are terminated locally. This is the path that
// makes video workable on DPI-blocked Google properties like YouTube.
type SNITunnel struct {
	CA *CA

	// UpstreamDial opens the outbound connection. For DPI bypass this
	// should wrap *fronter.Dialer so the outbound TLS carries a safe SNI.
	// The returned conn must behave as an already-handshaked TLS conn
	// (bytes written are encrypted on the wire, bytes read are plaintext).
	UpstreamDial func(ctx context.Context, network, addr string) (net.Conn, error)

	// UpstreamIP is the IP the tunnel dials. The port follows the browser's
	// CONNECT. For Google-edge-routed targets this is typically a Google
	// edge IP such as 216.239.38.120.
	UpstreamIP string

	// HandshakeTimeout bounds the browser-side TLS handshake. 0 = 15s.
	HandshakeTimeout time.Duration

	// Logger — nil uses slog.Default().
	Logger *slog.Logger

	leafMu    sync.Mutex
	leafCache map[string]*tls.Certificate
}

const defaultSNIHandshakeTimeout = 15 * time.Second

func (s *SNITunnel) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

// Tunnel takes a rawConn the browser CONNECTed through (SOCKS5-era),
// terminates its TLS with a leaf for host, opens the fronted upstream,
// and pipes bytes until either side closes. Closes rawConn on return.
func (s *SNITunnel) Tunnel(ctx context.Context, rawConn net.Conn, host string, port uint16) error {
	defer rawConn.Close()
	if s.UpstreamDial == nil {
		return errors.New("mitm: SNITunnel.UpstreamDial not configured")
	}
	if s.UpstreamIP == "" {
		return errors.New("mitm: SNITunnel.UpstreamIP not configured")
	}

	leaf, err := s.leafFor(host)
	if err != nil {
		return fmt.Errorf("mitm: SNI tunnel leaf: %w", err)
	}
	tlsConn := tls.Server(rawConn, &tls.Config{
		Certificates: []tls.Certificate{*leaf},
		MinVersion:   tls.VersionTLS12,
		// h1-only — upstream needs plain HTTP framing to be forwardable
		// as-is; h2 would need per-stream muxing we don't want to write.
		NextProtos: []string{"http/1.1"},
	})

	hsTimeout := s.HandshakeTimeout
	if hsTimeout == 0 {
		hsTimeout = defaultSNIHandshakeTimeout
	}
	_ = rawConn.SetDeadline(time.Now().Add(hsTimeout))
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return fmt.Errorf("mitm: SNI tunnel handshake: %w", err)
	}
	_ = rawConn.SetDeadline(time.Time{})

	upstream, err := s.UpstreamDial(ctx, "tcp", net.JoinHostPort(s.UpstreamIP, strconv.Itoa(int(port))))
	if err != nil {
		return fmt.Errorf("mitm: SNI tunnel upstream dial: %w", err)
	}
	defer upstream.Close()

	errc := make(chan error, 2)
	go func() {
		_, err := io.Copy(upstream, tlsConn)
		errc <- err
	}()
	go func() {
		_, err := io.Copy(tlsConn, upstream)
		errc <- err
	}()
	<-errc
	return nil
}

func (s *SNITunnel) leafFor(host string) (*tls.Certificate, error) {
	s.leafMu.Lock()
	defer s.leafMu.Unlock()
	if cert, ok := s.leafCache[host]; ok {
		return cert, nil
	}
	cert, err := s.CA.Sign(host)
	if err != nil {
		return nil, err
	}
	if s.leafCache == nil {
		s.leafCache = map[string]*tls.Certificate{}
	}
	s.leafCache[host] = cert
	return cert, nil
}
