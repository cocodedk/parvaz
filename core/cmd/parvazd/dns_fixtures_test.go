package main

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/cocodedk/parvaz/core/doh"
)

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// stubDoHServer returns any fixed response to every POST /dns-query.
// The dnsHandler forwards the response body verbatim (after ID restore
// and optional SERVFAIL/AAAA substitution), which is exactly what
// tests assert on.
func stubDoHServer(t *testing.T, response []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/dns-message")
		_, _ = w.Write(response)
	}))
}

func newTestHandler(endpoint string) *dnsHandler {
	return &dnsHandler{
		syntheticIP: "10.0.0.2",
		client:      &doh.Client{HTTP: http.DefaultClient, Endpoint: endpoint},
		logger:      silentLogger(),
	}
}

func fakeDNSQuery(txID byte) []byte {
	return []byte{
		txID, 0xCD,
		0x01, 0x00,
		0x00, 0x01,
		0x00, 0x00,
		0x00, 0x00,
		0x00, 0x00,
		0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		0x03, 'c', 'o', 'm',
		0x00,
		0x00, 0x01, 0x00, 0x01, // QTYPE=A, QCLASS=IN
	}
}

func fakeAAAAQuery(txID byte) []byte {
	return []byte{
		txID, 0x01,
		0x01, 0x00,
		0x00, 0x01,
		0x00, 0x00,
		0x00, 0x00,
		0x00, 0x00,
		0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		0x03, 'c', 'o', 'm',
		0x00,
		0x00, 0x1C, 0x00, 0x01, // QTYPE=AAAA(28), QCLASS=IN
	}
}

func fakeDNSResponse(txID byte) []byte {
	return []byte{
		txID, 0xEE,
		0x81, 0x80,
		0x00, 0x01,
		0x00, 0x01,
		0x00, 0x00,
		0x00, 0x00,
		0xDE, 0xAD, 0xBE, 0xEF,
	}
}
