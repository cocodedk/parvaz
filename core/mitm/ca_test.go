package mitm

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCA_GenerateAndPersist(t *testing.T) {
	dir := t.TempDir()

	first, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}
	if first.Cert == nil || first.Key == nil {
		t.Fatal("CA missing cert or key")
	}
	if !first.Cert.IsCA {
		t.Error("generated cert is not a CA (BasicConstraints.IsCA=false)")
	}
	if first.Cert.Subject.CommonName == "" {
		t.Error("CA has empty CommonName")
	}
	if first.Cert.KeyUsage&x509.KeyUsageCertSign == 0 {
		t.Error("CA lacks KeyUsageCertSign")
	}
	if ttl := time.Until(first.Cert.NotAfter); ttl < 365*24*time.Hour {
		t.Errorf("CA expires in %v, expected at least a year", ttl)
	}

	// Files must exist, be PEM-decodable, and the key file must be mode 0600.
	crtPath := filepath.Join(dir, "ca.crt")
	keyPath := filepath.Join(dir, "ca.key")
	crtBytes, err := os.ReadFile(crtPath)
	if err != nil {
		t.Fatalf("read ca.crt: %v", err)
	}
	if b, _ := pem.Decode(crtBytes); b == nil || b.Type != "CERTIFICATE" {
		t.Errorf("ca.crt not a PEM CERTIFICATE: %v", b)
	}
	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read ca.key: %v", err)
	}
	if b, _ := pem.Decode(keyBytes); b == nil {
		t.Errorf("ca.key not PEM-decodable")
	}
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("ca.key mode = %o, want 0600", info.Mode().Perm())
	}

	// Second call must reload the same material, not regenerate.
	second, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !bytes.Equal(first.Cert.Raw, second.Cert.Raw) {
		t.Error("reloaded CA cert differs from generated one")
	}
	if first.Key.D.Cmp(second.Key.D) != 0 {
		t.Error("reloaded CA key differs from generated one")
	}
}
