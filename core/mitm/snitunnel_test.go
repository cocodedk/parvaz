package mitm

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"slices"
	"testing"
	"time"
)

// sniTunnelRig is what dialSNITunnel returns — collects the handshaked
// client, the mock upstream peer, a channel that delivers whatever addr
// SNITunnel's UpstreamDial was called with, and a done channel for the
// Tunnel goroutine's exit.
type sniTunnelRig struct {
	tlsClient *tls.Conn
	upstream  net.Conn
	dialedTo  chan string
	done      chan struct{}
}

// dialSNITunnel spins up a SNITunnel behind a net.Pipe, performs the
// browser-side TLS handshake, and returns a rig the test can inspect.
// UpstreamDial sends the addr it was called with on a buffered channel
// so reads from the test goroutine don't race with the Tunnel goroutine.
func dialSNITunnel(t *testing.T, host string, port uint16, upstreamIP string) *sniTunnelRig {
	t.Helper()

	ca, err := LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	upServer, upClient := net.Pipe()
	dialedTo := make(chan string, 1)
	st := &SNITunnel{
		CA: ca,
		UpstreamDial: func(_ context.Context, _, addr string) (net.Conn, error) {
			dialedTo <- addr
			return upServer, nil
		},
		UpstreamIP: upstreamIP,
	}

	serverSide, clientSide := net.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = st.Tunnel(context.Background(), serverSide, host, port)
	}()

	roots := x509.NewCertPool()
	roots.AddCert(ca.Cert)
	tlsClient := tls.Client(clientSide, &tls.Config{
		RootCAs:    roots,
		ServerName: host,
	})
	_ = tlsClient.SetDeadline(time.Now().Add(5 * time.Second))
	if err := tlsClient.Handshake(); err != nil {
		_ = tlsClient.Close()
		_ = upClient.Close()
		t.Fatalf("TLS handshake through SNITunnel: %v", err)
	}
	return &sniTunnelRig{
		tlsClient: tlsClient,
		upstream:  upClient,
		dialedTo:  dialedTo,
		done:      done,
	}
}

func TestSNITunnel_BrowserTLSHandshake(t *testing.T) {
	r := dialSNITunnel(t, "m.youtube.com", 443, "216.239.38.120")
	defer func() {
		_ = r.tlsClient.Close()
		_ = r.upstream.Close()
		<-r.done
	}()

	state := r.tlsClient.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		t.Fatal("no peer cert")
	}
	leaf := state.PeerCertificates[0]
	if !slices.Contains(leaf.DNSNames, "m.youtube.com") {
		t.Errorf("leaf DNSNames = %v, want m.youtube.com", leaf.DNSNames)
	}
	if state.NegotiatedProtocol != "" && state.NegotiatedProtocol != "http/1.1" {
		t.Errorf("ALPN = %q, want http/1.1 (or empty)", state.NegotiatedProtocol)
	}
	select {
	case addr := <-r.dialedTo:
		if addr != "216.239.38.120:443" {
			t.Errorf("upstream dialed %q, want 216.239.38.120:443", addr)
		}
	case <-time.After(time.Second):
		t.Fatal("UpstreamDial not called within 1s of handshake")
	}
}

func TestSNITunnel_PipesPlaintextBidi(t *testing.T) {
	r := dialSNITunnel(t, "m.youtube.com", 443, "216.239.38.120")
	defer func() {
		_ = r.tlsClient.Close()
		_ = r.upstream.Close()
		<-r.done
	}()
	// Drain dialedTo so the Tunnel goroutine's send isn't stranded.
	<-r.dialedTo
	_ = r.tlsClient.SetDeadline(time.Now().Add(5 * time.Second))
	_ = r.upstream.SetDeadline(time.Now().Add(5 * time.Second))

	// Client → Upstream: write via tls (auto-encrypts), read plaintext at upstream.
	payload := []byte("GET / HTTP/1.1\r\nHost: m.youtube.com\r\n\r\n")
	go func() { _, _ = r.tlsClient.Write(payload) }()
	buf := make([]byte, len(payload))
	if _, err := io.ReadFull(r.upstream, buf); err != nil {
		t.Fatalf("upstream read: %v", err)
	}
	if !bytes.Equal(buf, payload) {
		t.Errorf("upstream got %q, want %q", buf, payload)
	}

	// Upstream → Client: write plaintext at upstream, read via tls at client.
	resp := []byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nhi")
	go func() { _, _ = r.upstream.Write(resp) }()
	got := make([]byte, len(resp))
	if _, err := io.ReadFull(r.tlsClient, got); err != nil {
		t.Fatalf("client read: %v", err)
	}
	if !bytes.Equal(got, resp) {
		t.Errorf("client got %q, want %q", got, resp)
	}
}

func TestSNITunnel_ClosesOnClientDisconnect(t *testing.T) {
	r := dialSNITunnel(t, "m.youtube.com", 443, "216.239.38.120")
	defer r.upstream.Close()
	<-r.dialedTo // drain

	// Closing the client tls conn should cascade: SNITunnel sees EOF on its
	// tls.Server side, closes upstream, pipes unblock, Tunnel returns.
	_ = r.tlsClient.Close()

	select {
	case <-r.done:
		_ = r.upstream.SetDeadline(time.Now().Add(500 * time.Millisecond))
		buf := make([]byte, 1)
		if _, err := r.upstream.Read(buf); err == nil {
			t.Error("upstream read returned data after client close, want EOF")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Tunnel did not exit within 2s after client close")
	}
}
