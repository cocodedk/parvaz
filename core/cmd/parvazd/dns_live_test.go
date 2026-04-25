package main

import (
	"context"
	"encoding/binary"
	"net"
	"os"
	"testing"
	"time"

	"github.com/cocodedk/parvaz/core/doh"
	"github.com/cocodedk/parvaz/core/fronter"
	"github.com/cocodedk/parvaz/core/relay"
)

// Live end-to-end DNS test. Walks the full production data plane:
//
//	dnsHandler.Handle → doh.Client.Query → relayRoundTripper → relay.Do
//	→ fronter → Google edge (216.239.38.120) → Apps Script → dns.google
//	→ back.
//
// Skipped unless PARVAZ_LIVE=1 + PARVAZ_LIVE_DEPLOYMENT_ID + PARVAZ_LIVE_AUTH_KEY
// are set. Consumes exactly one Apps Script quota unit per run.
//
//	source scripts/e2e/live.env
//	export PARVAZ_LIVE=1 PARVAZ_LIVE_DEPLOYMENT_ID PARVAZ_LIVE_AUTH_KEY
//	go test -C core -v -run TestDNS_Live ./cmd/parvazd/...
func TestDNS_Live_ResolvesExampleComViaRelay(t *testing.T) {
	if os.Getenv("PARVAZ_LIVE") != "1" {
		t.Skip("live test: set PARVAZ_LIVE=1 to run")
	}
	deploymentID := os.Getenv("PARVAZ_LIVE_DEPLOYMENT_ID")
	authKey := os.Getenv("PARVAZ_LIVE_AUTH_KEY")
	if deploymentID == "" || authKey == "" {
		t.Skip("live test: set PARVAZ_LIVE_DEPLOYMENT_ID + PARVAZ_LIVE_AUTH_KEY to run")
	}

	scriptURL := "https://script.google.com/macros/s/" + deploymentID + "/exec"
	d := &fronter.Dialer{
		FrontDomain:      "www.google.com",
		DialTimeout:      30 * time.Second,
		HandshakeTimeout: 30 * time.Second,
	}
	target := net.JoinHostPort("216.239.38.120", "443")
	rel, err := relay.New(relay.Config{
		HTTPClient: fronter.NewHTTPClient(d, target),
		ScriptURLs: []string{scriptURL},
		AuthKey:    authKey,
	})
	if err != nil {
		t.Fatalf("relay.New: %v", err)
	}

	h := &dnsHandler{
		syntheticIP: "10.0.0.2",
		client: &doh.Client{
			HTTP:     newDoHHTTPClient(rel),
			Endpoint: doh.DefaultEndpoint,
		},
		logger: silentLogger(),
	}
	// Loosen the default 3s DoH timeout — relay-via-Apps-Script adds
	// significant latency on top of the direct DoH POST.
	h.client.HTTP.Timeout = 30 * time.Second

	query := fakeDNSQuery(0xAB)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := h.Handle(ctx, "10.0.0.2", 53, query)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(resp) < 12 {
		t.Fatalf("response too short: %d bytes", len(resp))
	}
	if resp[0] != query[0] || resp[1] != query[1] {
		t.Errorf("ID not restored: got %02x%02x, want %02x%02x",
			resp[0], resp[1], query[0], query[1])
	}
	if resp[2]&0x80 == 0 {
		t.Error("QR bit not set")
	}
	if rcode := resp[3] & 0x0F; rcode != 0 {
		t.Errorf("RCODE = %d, want 0 (NOERROR); SERVFAIL means the relay path is broken", rcode)
	}
	ancount := binary.BigEndian.Uint16(resp[6:8])
	if ancount == 0 {
		t.Error("ANCOUNT = 0; expected ≥1 A record for example.com")
	}
	t.Logf("live DNS via relay: ANCOUNT=%d, %d bytes", ancount, len(resp))
}
