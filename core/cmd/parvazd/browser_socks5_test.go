package main

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

// resolveViaSOCKS5 runs the full UDP ASSOCIATE dance against the proxy
// and returns the A records the server answers with. Used by the live
// browser test — the SOCKS5 UDP side already has unit coverage in
// core/socks5, but wiring it together against a live server exercises
// the full handler chain.
func resolveViaSOCKS5(t *testing.T, proxyAddr, domain string) []net.IP {
	t.Helper()
	tcp, err := net.Dial("tcp", proxyAddr)
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer tcp.Close()
	_ = tcp.SetDeadline(time.Now().Add(20 * time.Second))
	if _, err := tcp.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		t.Fatalf("negotiate write: %v", err)
	}
	neg := make([]byte, 2)
	if _, err := io.ReadFull(tcp, neg); err != nil {
		t.Fatalf("negotiate read: %v", err)
	}
	if _, err := tcp.Write([]byte{0x05, 0x03, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
		t.Fatalf("associate write: %v", err)
	}
	reply := make([]byte, 10)
	if _, err := io.ReadFull(tcp, reply); err != nil {
		t.Fatalf("associate read: %v", err)
	}
	if reply[1] != 0 {
		t.Fatalf("UDP ASSOCIATE REP = %d, want 0", reply[1])
	}
	bindIP := net.IPv4(reply[4], reply[5], reply[6], reply[7])
	bindPort := binary.BigEndian.Uint16(reply[8:10])

	udp, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: bindIP, Port: int(bindPort)})
	if err != nil {
		t.Fatalf("dial udp: %v", err)
	}
	defer udp.Close()

	// SOCKS5 UDP header (dst=synthetic DNS 10.0.0.2:53) + DNS query.
	dgram := []byte{0, 0, 0, 0x01, 10, 0, 0, 2, 0, 53}
	dgram = append(dgram, dnsQueryA(0x42, domain)...)
	if _, err := udp.Write(dgram); err != nil {
		t.Fatalf("udp write: %v", err)
	}
	_ = udp.SetReadDeadline(time.Now().Add(20 * time.Second))
	buf := make([]byte, 2048)
	n, err := udp.Read(buf)
	if err != nil {
		t.Fatalf("udp read: %v", err)
	}
	body := stripSOCKSUDPHeader(t, buf[:n])
	return extractAAnswers(t, body)
}

// fetchViaSOCKS5 opens CONNECT to <domain>:443, TLS-handshakes using the
// proxy's CA as trust root, then HTTP GET / with Host: <domain>.
func fetchViaSOCKS5(t *testing.T, proxyAddr string, roots *x509.CertPool, domain string) *http.Response {
	t.Helper()
	tcp, err := net.Dial("tcp", proxyAddr)
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	_ = tcp.SetDeadline(time.Now().Add(30 * time.Second))
	if _, err := tcp.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		t.Fatalf("negotiate: %v", err)
	}
	neg := make([]byte, 2)
	if _, err := io.ReadFull(tcp, neg); err != nil {
		t.Fatalf("negotiate read: %v", err)
	}
	req := []byte{0x05, 0x01, 0x00, 0x03, byte(len(domain))}
	req = append(req, domain...)
	req = append(req, 0x01, 0xBB) // :443
	if _, err := tcp.Write(req); err != nil {
		t.Fatalf("connect write: %v", err)
	}
	connectReply := make([]byte, 10)
	if _, err := io.ReadFull(tcp, connectReply); err != nil {
		t.Fatalf("connect read: %v", err)
	}
	if connectReply[1] != 0 {
		t.Fatalf("CONNECT REP = %d, want 0", connectReply[1])
	}

	tlsConn := tls.Client(tcp, &tls.Config{RootCAs: roots, ServerName: domain})
	if err := tlsConn.Handshake(); err != nil {
		t.Fatalf("TLS handshake: %v", err)
	}
	if _, err := fmt.Fprintf(tlsConn,
		"GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", domain); err != nil {
		t.Fatalf("http write: %v", err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(tlsConn), nil)
	if err != nil {
		t.Fatalf("http read: %v", err)
	}
	return resp
}
