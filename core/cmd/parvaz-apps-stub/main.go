// parvaz-apps-stub is a standalone TLS server that mimics the Apps
// Script envelope contract (reference/apps_script/Code.gs) so the e2e
// harness can exercise the full traffic pipeline without a deployed
// Google Apps Script. The same handler code powers every relay unit
// test (core/internal/testutil), so protocol drift is impossible.
//
// Wire:
//
//	parvazd ──TLS(SNI=e2e.parvaz.test)── parvaz-apps-stub
//	        POST /macros/s/STUB1/exec { k, m, u, b?, ct?, h?, r }
//	        200  { s, h, b }
//
// Self-signs a TLS cert at startup; parvazd must run with -insecure-tls
// (planned flag) pointing at this stub's listen address.
package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cocodedk/parvaz/core/internal/testutil"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		log.SetFlags(0)
		log.Fatalf("parvaz-apps-stub: %v", err)
	}
}

func run() error {
	var (
		listen   = flag.String("listen", "127.0.0.1:8443", "TLS listen address (host:port)")
		authKey  = flag.String("auth-key", "e2e-test-key", "shared secret — must match parvazd's auth_key")
		sni      = flag.String("sni", "e2e.parvaz.test", "hostname the self-signed leaf cert is valid for")
		ver      = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()
	if *ver {
		fmt.Println("parvaz-apps-stub", version)
		return nil
	}

	stub := testutil.NewHandlerStub(*authKey)
	// Seed with one canned route so a bare `curl` smoke-tests the flow.
	// The harness or the caller is expected to populate additional routes
	// via a config reload (future) or by editing this file for bespoke
	// scenarios.
	stub.Routes["GET https://e2e.parvaz.test/hi"] = testutil.StubResponse{
		Status: 200,
		Header: map[string]string{"Content-Type": "text/plain"},
		Body:   []byte("hello from stub"),
	}

	cert, err := selfSignedCert(*sni)
	if err != nil {
		return fmt.Errorf("self-sign: %w", err)
	}
	srv := &http.Server{
		Handler:   stub.Handler(),
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
	}

	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		return fmt.Errorf("listen %s: %w", *listen, err)
	}
	defer ln.Close()

	fmt.Fprintf(os.Stdout, "READY %s sni=%s\n", ln.Addr().String(), *sni)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 1)
	go func() { errc <- srv.ServeTLS(ln, "", "") }()

	select {
	case err := <-errc:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}
	return nil
}

func selfSignedCert(sni string) (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, err
	}
	now := time.Now().UTC()
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: sni},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{sni},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return tls.Certificate{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return tls.X509KeyPair(certPEM, keyPEM)
}
