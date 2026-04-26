package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// TestBuildPipeline_MITMHandshake proves the full wire actually flows:
//
//	raw TCP → socks5 server → dispatcher → MITM path → interceptor's
//	tls.Server → leaf cert signed by a CA written under cfg.DataDir.
//
// The relay is not exercised (would require a live stub) — the TLS
// handshake alone is what proves every piece from flags through to the
// interceptor is wired correctly. A broken handshake would surface any
// of: missing DataDir, missing CA, dispatcher not installed, interceptor
// not mounted, pipe lifecycle broken, etc.
func TestBuildPipeline_MITMHandshake(t *testing.T) {
	cfg := Config{
		ScriptURLs:  []string{"https://stub.invalid/macros/s/X/exec"},
		AuthKey:     "test-key",
		GoogleIP:    "127.0.0.1",
		FrontDomain: "www.google.com",
		ListenHost:  "127.0.0.1",
		ListenPort:  0,
		DataDir:     t.TempDir(),
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	srv, cleanup, err := buildPipeline(cfg, logger)
	if err != nil {
		t.Fatalf("buildPipeline: %v", err)
	}
	defer cleanup()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Serve(ctx, ln) //nolint:errcheck

	// Load the CA the pipeline just generated.
	caBytes, err := os.ReadFile(filepath.Join(cfg.DataDir, "ca", "ca.crt"))
	if err != nil {
		t.Fatalf("read CA: %v", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caBytes) {
		t.Fatal("CA not PEM-parseable")
	}

	// SOCKS5 CONNECT to a non-Google host — forces the MITM path.
	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	socks5ConnectOrFatal(t, c, "netflic.com", 443)

	// TLS handshake against the CA — the interceptor must serve a leaf
	// for netflic.com signed by the CA.
	tlsClient := tls.Client(c, &tls.Config{
		RootCAs:    roots,
		ServerName: "netflic.com",
	})
	defer tlsClient.Close()
	if err := tlsClient.Handshake(); err != nil {
		t.Fatalf("TLS handshake through the pipeline failed: %v", err)
	}
	state := tlsClient.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		t.Fatal("no peer certs after handshake")
	}
	leaf := state.PeerCertificates[0]
	if !slices.Contains(leaf.DNSNames, "netflic.com") {
		t.Errorf("leaf DNSNames = %v, want netflic.com among them", leaf.DNSNames)
	}
}

// TestBuildPipeline_LegacySOCKS_NoDNSHandler ensures the codex-review P3
// regression stays fixed: running parvazd without a TUN fd (the
// pre-M15b mode still used by the apps-stub E2E) must not wire the
// synthetic DNS handler. Otherwise UDP ASSOCIATE would succeed but then
// silently drop packets for real DNS targets like 8.8.8.8:53, which is
// worse than the pre-change "REP=0x07 command not supported".
func TestBuildPipeline_LegacySOCKS_NoDNSHandler(t *testing.T) {
	cfg := Config{
		ScriptURLs:  []string{"https://stub.invalid/macros/s/X/exec"},
		AuthKey:     "test-key",
		GoogleIP:    "127.0.0.1",
		FrontDomain: "www.google.com",
		ListenHost:  "127.0.0.1",
		ListenPort:  0,
		DataDir:     t.TempDir(),
		// TunFD deliberately 0 — legacy SOCKS mode.
	}
	srv, cleanup, err := buildPipeline(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("buildPipeline: %v", err)
	}
	defer cleanup()
	if srv.Datagram != nil {
		t.Error("Datagram wired in TunFD=0 mode — DNS handler leaking into legacy SOCKS path")
	}
}

// TestBuildPipeline_AndroidSCMRights_DNSWired pins the bug fix for the
// Android SCM_RIGHTS handoff: when Kotlin sends TunFD=-1 (sentinel for
// "fd will arrive post-spawn over the abstract socket"), buildPipeline
// runs BEFORE recvTunFD assigns a real fd. The DNS handler must still
// be wired now — otherwise tun2socks's UDP path gets REP=0x07 from
// SOCKS5 UDP ASSOCIATE and browsers see ERR_NAME_NOT_RESOLVED.
func TestBuildPipeline_AndroidSCMRights_DNSWired(t *testing.T) {
	cfg := Config{
		ScriptURLs:  []string{"https://stub.invalid/macros/s/X/exec"},
		AuthKey:     "test-key",
		GoogleIP:    "127.0.0.1",
		FrontDomain: "www.google.com",
		ListenHost:  "127.0.0.1",
		ListenPort:  0,
		DataDir:     t.TempDir(),
		TunFD:       -1, // Android SCM_RIGHTS sentinel.
	}
	srv, cleanup, err := buildPipeline(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("buildPipeline: %v", err)
	}
	defer cleanup()
	if srv.Datagram == nil {
		t.Fatal("Datagram nil with TunFD=-1 — DNS gate broken for SCM_RIGHTS path")
	}
}

// socks5ConnectOrFatal performs a no-auth SOCKS5 negotiation and a CONNECT
// to host:port, t.Fatals on any wire-level failure. Exits when the server
// has written its replyOK (10 bytes starting 0x05,0x00).
func socks5ConnectOrFatal(t *testing.T, c net.Conn, host string, port uint16) {
	t.Helper()
	// [VER=5, NMETHODS=1, METHODS={0x00 no-auth}]
	if _, err := c.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		t.Fatalf("socks5 negotiate write: %v", err)
	}
	negReply := make([]byte, 2)
	if _, err := io.ReadFull(c, negReply); err != nil {
		t.Fatalf("socks5 negotiate read: %v", err)
	}
	if negReply[0] != 0x05 || negReply[1] != 0x00 {
		t.Fatalf("socks5 negotiate reply = %v, want [0x05 0x00]", negReply)
	}
	// [VER=5, CMD=CONNECT, RSV=0, ATYP=3 domain, LEN, DOMAIN, PORT]
	req := []byte{0x05, 0x01, 0x00, 0x03, byte(len(host))}
	req = append(req, host...)
	portBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(portBuf, port)
	req = append(req, portBuf...)
	if _, err := c.Write(req); err != nil {
		t.Fatalf("socks5 CONNECT write: %v", err)
	}
	resp := make([]byte, 10)
	if _, err := io.ReadFull(c, resp); err != nil {
		t.Fatalf("socks5 CONNECT read: %v", err)
	}
	if resp[0] != 0x05 || resp[1] != 0x00 {
		t.Fatalf("socks5 CONNECT reply = %v, want VER=5 REP=0", resp)
	}
}
