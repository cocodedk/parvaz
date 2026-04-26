package fronter

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newTLSHTTPServer returns an in-process HTTPS server whose TLS layer records
// the SNI of each ClientHello. The http.Handler is user-supplied.
func newTLSHTTPServer(t *testing.T, sni *string, handler http.Handler) *httptest.Server {
	t.Helper()
	cert := selfSignedCert(t)
	srv := httptest.NewUnstartedServer(handler)
	srv.TLS = &tls.Config{
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			*sni = hello.ServerName
			return &cert, nil
		},
	}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv
}

func TestClient_SendsHostHeaderOverridingSNI(t *testing.T) {
	var observedSNI, observedHost string
	srv := newTLSHTTPServer(t, &observedSNI,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			observedHost = r.Host
			_, _ = w.Write([]byte("ok"))
		}))

	d := &Dialer{FrontDomain: "www.google.com", InsecureSkipVerify: true}
	client := NewHTTPClient(d, srv.Listener.Addr().String())
	req, _ := http.NewRequest(http.MethodGet, "https://script.google.com/macros/s/X/exec", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	_ = resp.Body.Close()

	if observedSNI != "www.google.com" {
		t.Errorf("SNI = %q, want www.google.com (dialed %s)", observedSNI, srv.Listener.Addr())
	}
	if observedHost != "script.google.com" {
		t.Errorf("Host header = %q, want script.google.com", observedHost)
	}
}

func TestClient_POSTJSONBody_EchoedBack(t *testing.T) {
	var sni string
	payload := []byte(`{"k":"test","m":"GET"}`)
	srv := newTLSHTTPServer(t, &sni,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			if r.Header.Get("Content-Type") != "application/json" {
				http.Error(w, "bad ct", 400)
				return
			}
			_, _ = w.Write(got)
		}))

	d := &Dialer{FrontDomain: "www.google.com", InsecureSkipVerify: true}
	client := NewHTTPClient(d, srv.Listener.Addr().String())
	req, _ := http.NewRequest(http.MethodPost, "https://script.google.com/x",
		bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()

	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("body echo mismatch: got %q want %q", got, payload)
	}
}

func TestClient_HandlesNonSuccessStatus(t *testing.T) {
	var sni string
	srv := newTLSHTTPServer(t, &sni,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("backend down"))
		}))

	d := &Dialer{FrontDomain: "www.google.com", InsecureSkipVerify: true}
	client := NewHTTPClient(d, srv.Listener.Addr().String())
	req, _ := http.NewRequest(http.MethodGet, "https://script.google.com/x", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "backend down" {
		t.Errorf("body = %q, want %q", body, "backend down")
	}
}

// Default net/http MaxIdleConnsPerHost is 2, which serializes every
// fronted POST behind two TLS sockets to the same Google edge IP. The
// fronted leg is the dominant per-request latency, so a small pool
// causes severe head-of-line blocking under any concurrency. Assert
// the fronter raises the pool to a reasonable size and caps total
// conns/host so we don't leak file descriptors either.
func TestNewHTTPClient_TransportPoolDefaults(t *testing.T) {
	d := &Dialer{FrontDomain: "www.google.com", InsecureSkipVerify: true}
	client := NewHTTPClient(d, "1.2.3.4:443")
	tr, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport type = %T, want *http.Transport", client.Transport)
	}
	if tr.MaxIdleConnsPerHost < 8 {
		t.Errorf("MaxIdleConnsPerHost = %d, want ≥ 8 (net/http default 2 causes HOL blocking)",
			tr.MaxIdleConnsPerHost)
	}
	if tr.MaxConnsPerHost == 0 || tr.MaxConnsPerHost < tr.MaxIdleConnsPerHost {
		t.Errorf("MaxConnsPerHost = %d, want ≥ MaxIdleConnsPerHost (%d)",
			tr.MaxConnsPerHost, tr.MaxIdleConnsPerHost)
	}
}

func TestClient_PropagatesContextDeadline(t *testing.T) {
	var sni string
	// Handler blocks longer than the client context — request must cancel.
	srv := newTLSHTTPServer(t, &sni,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-time.After(2 * time.Second):
				_, _ = w.Write([]byte("late"))
			case <-r.Context().Done():
			}
		}))

	d := &Dialer{FrontDomain: "www.google.com", InsecureSkipVerify: true}
	client := NewHTTPClient(d, srv.Listener.Addr().String())

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://script.google.com/x", nil)

	start := time.Now()
	_, err := client.Do(req)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected deadline error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Logf("error = %v (%T) — checking Timeout()", err, err)
	}
	if elapsed > time.Second {
		t.Errorf("did not cancel promptly: elapsed %s", elapsed)
	}
}
