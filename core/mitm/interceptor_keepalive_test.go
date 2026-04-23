package mitm

import (
	"bufio"
	"net/http"
	"testing"
	"time"

	"github.com/cocodedk/parvaz/core/protocol"
)

func TestInterceptor_KeepAlive_TwoRequestsOneConn(t *testing.T) {
	ca, err := LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rec := &stubRelay{
		response: &protocol.Response{Status: 200, Header: http.Header{}},
	}
	ic := &Interceptor{CA: ca, Relay: rec}

	tlsClient, done := dialTLSToInterceptor(t, ic, "example.com", 443)
	defer func() {
		_ = tlsClient.Close()
		<-done
	}()

	// Verify ALPN settled on h1 — without NextProtos on the server, Go's
	// defaults would negotiate h2 if the client advertised it, and the rest
	// of this test would fail with a framing error.
	if np := tlsClient.ConnectionState().NegotiatedProtocol; np != "" && np != "http/1.1" {
		t.Fatalf("negotiated ALPN = %q, want http/1.1 or empty", np)
	}

	br := bufio.NewReader(tlsClient)
	for i, path := range []string{"/first", "/second"} {
		req, err := http.NewRequest("GET", "https://example.com"+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		_ = tlsClient.SetDeadline(time.Now().Add(3 * time.Second))
		if err := req.Write(tlsClient); err != nil {
			t.Fatalf("req %d write: %v", i, err)
		}
		resp, err := http.ReadResponse(br, req)
		if err != nil {
			t.Fatalf("req %d read: %v", i, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("req %d status = %d", i, resp.StatusCode)
		}
	}

	rec.mu.Lock()
	hits := rec.hits
	lastURL := rec.lastReq.URL
	follow := rec.lastReq.FollowRedirects
	rec.mu.Unlock()
	if hits != 2 {
		t.Errorf("relay hits = %d, want 2 (keep-alive reused)", hits)
	}
	if lastURL != "https://example.com/second" {
		t.Errorf("last URL = %q, want /second", lastURL)
	}
	if follow {
		t.Error("FollowRedirects = true — browser should see 3xx itself")
	}
}
