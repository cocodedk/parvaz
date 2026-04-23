package fronter

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"testing"
	"time"
)

func selfSignedCert(t *testing.T) tls.Certificate {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: priv}
}

// newTLSRecorder returns a TLS listener on 127.0.0.1:0 that records the
// SNI of each ClientHello into *sni (via the cert callback).
func newTLSRecorder(t *testing.T, sni *string) net.Listener {
	t.Helper()
	cert := selfSignedCert(t)
	cfg := &tls.Config{
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			*sni = hello.ServerName
			return &cert, nil
		},
	}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", cfg)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				// Force handshake completion, then close.
				_ = c.(*tls.Conn).Handshake()
				_ = c.Close()
			}(c)
		}
	}()
	return ln
}

func TestDial_UsesCustomSNI(t *testing.T) {
	var observed string
	ln := newTLSRecorder(t, &observed)
	defer ln.Close()

	d := &Dialer{FrontDomain: "www.google.com", InsecureSkipVerify: true}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := d.Dial(ctx, "tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_ = conn.Close()

	if observed != "www.google.com" {
		t.Errorf("SNI = %q, want www.google.com (dialed %s)", observed, ln.Addr())
	}
}

func TestDial_ReturnsConnError_WhenUnreachable(t *testing.T) {
	// Bind + close to get a port the kernel will reject with ECONNREFUSED.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	d := &Dialer{FrontDomain: "www.google.com", InsecureSkipVerify: true}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = d.Dial(ctx, "tcp", addr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// The underlying TCP connect should fail; exact error shape is OS-dependent.
	var opErr *net.OpError
	if !errors.As(err, &opErr) {
		t.Logf("error type = %T: %v", err, err)
	}
}

func TestDial_RespectsContextCancellation(t *testing.T) {
	// Plain TCP listener — accepts but never serves any TLS bytes,
	// so the client handshake blocks until the context fires.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		c, _ := ln.Accept()
		if c != nil {
			accepted <- c
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	start := time.Now()
	d := &Dialer{FrontDomain: "www.google.com", InsecureSkipVerify: true}
	_, err = d.Dial(ctx, "tcp", ln.Addr().String())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
	if elapsed > time.Second {
		t.Errorf("dial did not cancel promptly: elapsed %s", elapsed)
	}

	// Clean up the hanging server-side accept.
	select {
	case c := <-accepted:
		_ = c.Close()
	case <-time.After(200 * time.Millisecond):
	}
}

func TestDial_HonorsHandshakeTimeout(t *testing.T) {
	// Plain TCP listener that accepts but never sends TLS bytes. The TLS
	// handshake blocks waiting for ServerHello; the Dialer's own
	// HandshakeTimeout must break it even when the caller context is Background.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		c, _ := ln.Accept()
		if c != nil {
			accepted <- c
		}
	}()

	d := &Dialer{
		FrontDomain:        "www.google.com",
		InsecureSkipVerify: true,
		HandshakeTimeout:   150 * time.Millisecond,
	}
	start := time.Now()
	_, err = d.Dial(context.Background(), "tcp", ln.Addr().String())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected handshake timeout, got nil")
	}
	if elapsed > time.Second {
		t.Errorf("handshake did not time out promptly: elapsed %s", elapsed)
	}

	select {
	case c := <-accepted:
		_ = c.Close()
	case <-time.After(200 * time.Millisecond):
	}
}
