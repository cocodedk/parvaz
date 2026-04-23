package doh

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeQuery builds the minimal valid DNS header + a single A query for
// "example.com". 12-byte header + qname + qtype + qclass.
func fakeQuery(txID byte) []byte {
	return []byte{
		txID, 0xCD, // ID
		0x01, 0x00, // flags: recursion desired
		0x00, 0x01, // QDCOUNT=1
		0x00, 0x00, // ANCOUNT
		0x00, 0x00, // NSCOUNT
		0x00, 0x00, // ARCOUNT
		// QNAME: example.com
		0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		0x03, 'c', 'o', 'm',
		0x00,
		// QTYPE = A (1), QCLASS = IN (1)
		0x00, 0x01, 0x00, 0x01,
	}
}

// fakeResponse mirrors the query and adds a minimal A record answer.
// We don't validate the record — only the ID handling.
func fakeResponse(txID byte) []byte {
	return []byte{
		txID, 0xEE, // ID — note different low byte so we can tell request from response
		0x81, 0x80, // flags: standard response, no error
		0x00, 0x01, // QDCOUNT
		0x00, 0x01, // ANCOUNT
		0x00, 0x00, // NSCOUNT
		0x00, 0x00, // ARCOUNT
		// answer body doesn't matter for the test — any trailing bytes work
		0xDE, 0xAD, 0xBE, 0xEF,
	}
}

func TestClient_Query_ZeroesIDOnRequestAndRestoresOnResponse(t *testing.T) {
	var seen []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/dns-message" {
			t.Errorf("Content-Type = %q, want application/dns-message", ct)
		}
		if ac := r.Header.Get("Accept"); !strings.Contains(ac, "application/dns-message") {
			t.Errorf("Accept = %q, want application/dns-message", ac)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Method = %q, want POST", r.Method)
		}
		b, _ := io.ReadAll(r.Body)
		seen = b
		w.Header().Set("Content-Type", "application/dns-message")
		_, _ = w.Write(fakeResponse(0x00)) // server returns with ID=0 per RFC 8484
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), Endpoint: srv.URL}
	resp, err := c.Query(context.Background(), fakeQuery(0xAB))
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if seen[0] != 0x00 || seen[1] != 0x00 {
		t.Errorf("request ID not zeroed: got %02x%02x", seen[0], seen[1])
	}
	if resp[0] != 0xAB || resp[1] != 0xCD {
		t.Errorf("response ID not restored to caller's: got %02x%02x, want ABCD", resp[0], resp[1])
	}
}

func TestClient_Query_RejectsTruncatedQuery(t *testing.T) {
	c := &Client{HTTP: http.DefaultClient, Endpoint: DefaultEndpoint}
	_, err := c.Query(context.Background(), []byte{0x00, 0x00})
	if err == nil || !strings.Contains(err.Error(), "too short") {
		t.Errorf("expected 'too short' error, got %v", err)
	}
}

func TestClient_Query_PropagatesNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream exploded", http.StatusBadGateway)
	}))
	defer srv.Close()
	c := &Client{HTTP: srv.Client(), Endpoint: srv.URL}
	_, err := c.Query(context.Background(), fakeQuery(0x01))
	if err == nil || !strings.Contains(err.Error(), "status 502") {
		t.Errorf("expected status-502 error, got %v", err)
	}
}

func TestClient_Query_RejectsTruncatedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/dns-message")
		_, _ = w.Write([]byte{0x00}) // 1 byte — too short to even have an ID
	}))
	defer srv.Close()
	c := &Client{HTTP: srv.Client(), Endpoint: srv.URL}
	_, err := c.Query(context.Background(), fakeQuery(0x02))
	if err == nil || !strings.Contains(err.Error(), "truncated response") {
		t.Errorf("expected truncated-response error, got %v", err)
	}
}

func TestClient_Query_UsesDefaultEndpointWhenEmpty(t *testing.T) {
	// We can't actually hit Google from a unit test; just assert the
	// empty-endpoint path substitutes DefaultEndpoint. We inject a
	// client whose transport fails fast so we observe the URL.
	var requestedURL string
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestedURL = req.URL.String()
		return nil, errConnectRefused
	})
	c := &Client{HTTP: &http.Client{Transport: rt}}
	_, _ = c.Query(context.Background(), fakeQuery(0x03))
	if requestedURL != DefaultEndpoint {
		t.Errorf("default endpoint not used: got %q, want %q", requestedURL, DefaultEndpoint)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type testError string

func (e testError) Error() string { return string(e) }

const errConnectRefused = testError("connect refused")
