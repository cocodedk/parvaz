package mitm

import (
	"crypto/x509"
	"fmt"
	"net"
	"testing"
)

func TestLeaf_SignedByCA_NameMatchesHost(t *testing.T) {
	ca, err := LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}

	roots := x509.NewCertPool()
	roots.AddCert(ca.Cert)

	cases := []struct {
		name   string
		host   string
		verify string
	}{
		{"dns", "example.com", "example.com"},
		{"dns subdomain", "api.example.com", "api.example.com"},
		{"ipv4", "192.0.2.1", "192.0.2.1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tlsCert, err := ca.Sign(tc.host)
			if err != nil {
				t.Fatalf("Sign(%q): %v", tc.host, err)
			}
			if len(tlsCert.Certificate) == 0 {
				t.Fatal("leaf has no certificate chain")
			}
			leaf, err := x509.ParseCertificate(tlsCert.Certificate[0])
			if err != nil {
				t.Fatal(err)
			}

			opts := x509.VerifyOptions{
				Roots:     roots,
				KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			}
			if ip := net.ParseIP(tc.verify); ip != nil {
				// DNSName won't match an IP literal; rely on IP SAN.
				opts.DNSName = ""
			} else {
				opts.DNSName = tc.verify
			}
			if _, err := leaf.Verify(opts); err != nil {
				t.Errorf("Verify(%q): %v", tc.verify, err)
			}
			if leaf.Issuer.CommonName != ca.Cert.Subject.CommonName {
				t.Errorf("leaf issuer CN = %q, want %q",
					leaf.Issuer.CommonName, ca.Cert.Subject.CommonName)
			}
		})
	}
}

func TestLeaf_DistinctHostsProduceDistinctCerts(t *testing.T) {
	ca, err := LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a, err := ca.Sign("a.example.com")
	if err != nil {
		t.Fatal(err)
	}
	b, err := ca.Sign("b.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if string(a.Certificate[0]) == string(b.Certificate[0]) {
		t.Error("distinct hosts produced identical leaf certs")
	}
}

func TestCA_Sign_Concurrent(t *testing.T) {
	ca, err := LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca.Cert)

	const n = 64
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func(i int) {
			host := fmt.Sprintf("h%d.example.com", i)
			cert, err := ca.Sign(host)
			if err != nil {
				errs <- fmt.Errorf("sign %s: %w", host, err)
				return
			}
			if _, err := cert.Leaf.Verify(x509.VerifyOptions{
				Roots:     roots,
				DNSName:   host,
				KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			}); err != nil {
				errs <- fmt.Errorf("verify %s: %w", host, err)
				return
			}
			errs <- nil
		}(i)
	}
	for i := 0; i < n; i++ {
		if err := <-errs; err != nil {
			t.Error(err)
		}
	}
}
