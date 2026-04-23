// Package mitm generates a private CA and per-host leaf certificates so a
// local TLS server can terminate browser HTTPS. The user installs the CA
// into Android's user-CA store once; thereafter the interceptor (other file)
// accepts the browser's TLS, parses the request, and hands it to the relay.
//
// ECDSA P-256 throughout. Android 7+ trusts ECDSA user-CAs, keys and
// certs are small, and signing is fast enough that per-connection leaves
// are cheap.
package mitm

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// CA holds the signing material for leaf certs. Safe for concurrent use.
type CA struct {
	Cert *x509.Certificate
	Key  *ecdsa.PrivateKey
}

const (
	caSubdir       = "ca"
	caCertFilename = "ca.crt"
	caKeyFilename  = "ca.key"

	caValidity = 10 * 365 * 24 * time.Hour // ~10 years
)

// LoadOrCreate loads a previously-persisted CA from dataDir/ca/, or
// generates and writes a new one. The `ca/` subdirectory nests the CA so
// dataDir can later hold a leaf cache, settings, logs, etc. without the
// CA at its root. Directory is 0700; key file 0600; cert file 0644
// (it's the public anchor).
func LoadOrCreate(dataDir string) (*CA, error) {
	caDir := filepath.Join(dataDir, caSubdir)
	if err := os.MkdirAll(caDir, 0o700); err != nil {
		return nil, fmt.Errorf("mitm: mkdir CA dir: %w", err)
	}
	crtPath := filepath.Join(caDir, caCertFilename)
	keyPath := filepath.Join(caDir, caKeyFilename)

	if _, err := os.Stat(crtPath); err == nil {
		return loadCA(crtPath, keyPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("mitm: stat CA cert: %w", err)
	}
	return generateCA(crtPath, keyPath)
}

func loadCA(crtPath, keyPath string) (*CA, error) {
	crtBytes, err := os.ReadFile(crtPath)
	if err != nil {
		return nil, fmt.Errorf("mitm: read CA cert: %w", err)
	}
	block, _ := pem.Decode(crtBytes)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("mitm: CA cert not PEM CERTIFICATE")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("mitm: parse CA cert: %w", err)
	}

	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("mitm: read CA key: %w", err)
	}
	kblock, _ := pem.Decode(keyBytes)
	if kblock == nil {
		return nil, fmt.Errorf("mitm: CA key not PEM")
	}
	key, err := x509.ParseECPrivateKey(kblock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("mitm: parse CA key: %w", err)
	}
	return &CA{Cert: cert, Key: key}, nil
}

func generateCA(crtPath, keyPath string) (*CA, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("mitm: generate CA key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("mitm: serial: %w", err)
	}
	now := time.Now().UTC()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "Parvaz Root CA",
			Organization: []string{"Parvaz"},
		},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(caValidity),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true, // don't allow sub-CAs
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("mitm: create CA cert: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("mitm: parse newly-minted CA cert: %w", err)
	}

	if err := writePEM(crtPath, "CERTIFICATE", der, 0o644); err != nil {
		return nil, err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("mitm: marshal CA key: %w", err)
	}
	if err := writePEM(keyPath, "EC PRIVATE KEY", keyDER, 0o600); err != nil {
		return nil, err
	}
	return &CA{Cert: cert, Key: key}, nil
}

func writePEM(path, blockType string, der []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("mitm: open %s: %w", path, err)
	}
	defer f.Close()
	if err := pem.Encode(f, &pem.Block{Type: blockType, Bytes: der}); err != nil {
		return fmt.Errorf("mitm: write %s: %w", path, err)
	}
	return nil
}

// PEM returns the CA certificate in PEM form — the byte stream the Android
// side hands to ACTION_MANAGE_CA_CERTIFICATES.
func (c *CA) PEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Cert.Raw})
}
