package mitm

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/cocodedk/parvaz/core/protocol"
)

// stubRelay captures the last request it saw and returns Response (or Err).
// Safe for concurrent use by a single goroutine sender + a single reader,
// which is what the interceptor does per-connection.
type stubRelay struct {
	mu       sync.Mutex
	lastReq  protocol.Request
	response *protocol.Response
	err      error
	hits     int
}

func (s *stubRelay) Do(_ context.Context, req protocol.Request) (*protocol.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Drain body now; the caller owns the underlying buffer only briefly.
	cp := req
	if req.Body != nil {
		cp.Body = append([]byte(nil), req.Body...)
	}
	s.lastReq = cp
	s.hits++
	if s.err != nil {
		return nil, s.err
	}
	if s.response == nil {
		return &protocol.Response{Status: 200, Header: http.Header{}, Body: nil}, nil
	}
	// Clone so the test's response template isn't mutated.
	clone := *s.response
	clone.Body = append([]byte(nil), s.response.Body...)
	return &clone, nil
}

// dialTLSToInterceptor pipes an Interceptor against an in-process TLS
// client. Returns the client-side tls.Conn plus a chan that receives the
// interceptor's exit error.
func dialTLSToInterceptor(t *testing.T, ic *Interceptor, host string, port uint16) (*tls.Conn, chan error) {
	t.Helper()
	serverSide, clientSide := net.Pipe()

	done := make(chan error, 1)
	go func() {
		done <- ic.Intercept(context.Background(), serverSide, host, port)
	}()

	roots := x509.NewCertPool()
	roots.AddCert(ic.CA.Cert)
	tlsClient := tls.Client(clientSide, &tls.Config{
		RootCAs:    roots,
		ServerName: host,
	})
	_ = tlsClient.SetDeadline(time.Now().Add(5 * time.Second))
	if err := tlsClient.Handshake(); err != nil {
		_ = tlsClient.Close()
		t.Fatalf("TLS client handshake: %v", err)
	}
	return tlsClient, done
}

func TestInterceptor_TLSServer_AcceptsBrowserClientUsingCA(t *testing.T) {
	ca, err := LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ic := &Interceptor{CA: ca, Relay: &stubRelay{}}

	tlsClient, done := dialTLSToInterceptor(t, ic, "example.com", 443)
	defer func() {
		_ = tlsClient.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("interceptor did not exit after client close")
		}
	}()

	state := tlsClient.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		t.Fatal("no peer certificates after handshake")
	}
	leaf := state.PeerCertificates[0]
	foundSAN := false
	for _, n := range leaf.DNSNames {
		if n == "example.com" {
			foundSAN = true
			break
		}
	}
	if !foundSAN {
		t.Errorf("leaf DNSNames = %v, want example.com among them", leaf.DNSNames)
	}
}

func TestInterceptor_ForwardsHTTPRequestThroughRelay(t *testing.T) {
	ca, err := LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rec := &stubRelay{
		response: &protocol.Response{
			Status: 204,
			Header: http.Header{"X-Seen": []string{"yes"}},
		},
	}
	ic := &Interceptor{CA: ca, Relay: rec}

	tlsClient, done := dialTLSToInterceptor(t, ic, "api.example.com", 443)
	defer func() {
		_ = tlsClient.Close()
		<-done
	}()

	body := []byte(`{"hello":"world"}`)
	req, err := http.NewRequest("POST", "https://api.example.com/path?x=1", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Custom", "parvaz")
	if err := req.Write(tlsClient); err != nil {
		t.Fatalf("write req: %v", err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(tlsClient), req)
	if err != nil {
		t.Fatalf("read resp: %v", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != 204 {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}
	if resp.Header.Get("X-Seen") != "yes" {
		t.Errorf("X-Seen header = %q, want yes", resp.Header.Get("X-Seen"))
	}

	rec.mu.Lock()
	got := rec.lastReq
	hits := rec.hits
	rec.mu.Unlock()
	if hits != 1 {
		t.Errorf("relay hits = %d, want 1", hits)
	}
	if got.Method != "POST" {
		t.Errorf("relay saw method %q, want POST", got.Method)
	}
	if got.URL != "https://api.example.com/path?x=1" {
		t.Errorf("relay saw URL %q", got.URL)
	}
	if got.Header.Get("X-Custom") != "parvaz" {
		t.Errorf("relay saw X-Custom = %q", got.Header.Get("X-Custom"))
	}
	if got.ContentType != "application/json" {
		t.Errorf("relay saw ContentType = %q", got.ContentType)
	}
	if !bytes.Equal(got.Body, body) {
		t.Errorf("relay saw body %q, want %q", got.Body, body)
	}
}

