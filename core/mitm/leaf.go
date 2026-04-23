package mitm

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"time"
)

const leafValidity = 365 * 24 * time.Hour // one year

// Sign produces a tls.Certificate for host, signed by the CA. host may be
// a DNS name or a bare IP literal; the SAN is set accordingly. A fresh
// ECDSA P-256 key is minted for each leaf.
func (c *CA) Sign(host string) (*tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("mitm: leaf key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("mitm: serial: %w", err)
	}
	now := time.Now().UTC()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: host},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(leafValidity),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	if ip := net.ParseIP(host); ip != nil {
		tmpl.IPAddresses = []net.IP{ip}
	} else {
		tmpl.DNSNames = []string{host}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, c.Cert, &key.PublicKey, c.Key)
	if err != nil {
		return nil, fmt.Errorf("mitm: sign leaf: %w", err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("mitm: parse newly-minted leaf: %w", err)
	}
	return &tls.Certificate{
		Certificate: [][]byte{der, c.Cert.Raw},
		PrivateKey:  key,
		Leaf:        parsed,
	}, nil
}
