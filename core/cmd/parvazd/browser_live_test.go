package main

import (
	"bytes"
	"context"
	"crypto/x509"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// TestBrowser_Live_CocodeDk walks the full production data plane end-to-end
// against a real website on the real internet (cocode.dk) — the closest
// approximation of "Chrome loading a page through Parvaz" we can run
// without an Android emulator. Exercises:
//
//  1. SOCKS5 UDP ASSOCIATE for DNS resolution of cocode.dk (via the DoH
//     shim → Apps Script relay → dns.google).
//  2. SOCKS5 CONNECT + MITM + relay + real-website fetch for the TCP
//     side (TLS handshake against the minted leaf, HTTP GET /, body
//     assertion).
//
// Skipped unless PARVAZ_LIVE=1 + deployment-id + auth-key are set. Each
// run consumes ≥2 Apps Script quota units (one for DNS, one for GET).
// Test client helpers (SOCKS5 UDP / CONNECT, DNS wire parsing) live in
// browser_live_helpers_test.go.
//
//	source scripts/e2e/live.env
//	export PARVAZ_LIVE=1 PARVAZ_LIVE_DEPLOYMENT_ID PARVAZ_LIVE_AUTH_KEY
//	go test -C core -v -run TestBrowser_Live ./cmd/parvazd/... -timeout 60s
func TestBrowser_Live_CocodeDk(t *testing.T) {
	if os.Getenv("PARVAZ_LIVE") != "1" {
		t.Skip("live test: set PARVAZ_LIVE=1 to run")
	}
	depID := os.Getenv("PARVAZ_LIVE_DEPLOYMENT_ID")
	authKey := os.Getenv("PARVAZ_LIVE_AUTH_KEY")
	if depID == "" || authKey == "" {
		t.Skip("live test: set PARVAZ_LIVE_DEPLOYMENT_ID + PARVAZ_LIVE_AUTH_KEY")
	}

	cfg := Config{
		ScriptURLs:  []string{"https://script.google.com/macros/s/" + depID + "/exec"},
		AuthKey:     authKey,
		GoogleIP:    "216.239.38.120",
		FrontDomain: "www.google.com",
		FrontPort:   443,
		ListenHost:  "127.0.0.1",
		ListenPort:  0,
		DataDir:     t.TempDir(),
		// Non-zero TunFD wires the DNS handler into the pipeline
		// (see buildPipeline). tun2socks itself is spawned in main.go
		// which this test doesn't touch — the fd value is never dereferenced.
		TunFD: 1,
	}
	srv, err := buildPipeline(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("buildPipeline: %v", err)
	}
	if srv.Datagram == nil {
		t.Fatal("DNS handler not wired — did TunFD gate change?")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Serve(ctx, ln) //nolint:errcheck

	caBytes, err := os.ReadFile(filepath.Join(cfg.DataDir, "ca", "ca.crt"))
	if err != nil {
		t.Fatalf("read CA: %v", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caBytes) {
		t.Fatal("CA not PEM-parseable")
	}
	proxyAddr := ln.Addr().String()

	// Step 1 — DNS via SOCKS5 UDP ASSOCIATE.
	ips := resolveViaSOCKS5(t, proxyAddr, "cocode.dk")
	if len(ips) == 0 {
		t.Fatal("DNS returned no A records for cocode.dk")
	}
	t.Logf("DNS: cocode.dk → %v", ips)

	// Step 2 — SOCKS5 CONNECT + MITM + TLS + HTTP GET /.
	resp := fetchViaSOCKS5(t, proxyAddr, roots, "cocode.dk")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	// cocode.dk renders the user's site; the word "Cocode" is a stable
	// identifier in the HTML across redesigns.
	if !bytes.Contains(bytes.ToLower(body), []byte("cocode")) {
		t.Errorf("body doesn't contain 'cocode'; first 200 bytes: %q",
			body[:min(200, len(body))])
	}
	t.Logf("HTTP: status=%d, body=%d bytes", resp.StatusCode, len(body))
}
