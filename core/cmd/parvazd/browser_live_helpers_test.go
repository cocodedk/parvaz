package main

import (
	"encoding/binary"
	"errors"
	"net"
	"strings"
	"testing"
)

// dnsQueryA assembles a minimal RD=1 A query for <domain>. Supports
// multi-label names ("cocode.dk") via simple split-on-dot.
func dnsQueryA(txID byte, domain string) []byte {
	hdr := []byte{
		txID, 0x00,
		0x01, 0x00, // RD=1
		0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	for _, label := range strings.Split(domain, ".") {
		hdr = append(hdr, byte(len(label)))
		hdr = append(hdr, label...)
	}
	hdr = append(hdr, 0x00)
	hdr = append(hdr, 0x00, 0x01, 0x00, 0x01) // QTYPE=A, QCLASS=IN
	return hdr
}

// stripSOCKSUDPHeader peels off the SOCKS5 UDP response header (RSV+FRAG+
// ATYP+DST.ADDR+DST.PORT) and returns the payload.
func stripSOCKSUDPHeader(t *testing.T, msg []byte) []byte {
	t.Helper()
	if len(msg) < 10 || msg[0] != 0 || msg[1] != 0 || msg[2] != 0 {
		t.Fatalf("bad SOCKS5 UDP header: %x", msg[:min(16, len(msg))])
	}
	off := 4
	switch msg[3] {
	case 0x01:
		off += 4
	case 0x03:
		off += 1 + int(msg[4])
	case 0x04:
		off += 16
	default:
		t.Fatalf("unsupported ATYP %d", msg[3])
	}
	return msg[off+2:]
}

// extractAAnswers scans a DNS response body for A records and returns
// their IPs. Tolerates compression pointers in RR NAME fields
// (RFC 1035 §4.1.4).
func extractAAnswers(t *testing.T, msg []byte) []net.IP {
	t.Helper()
	if len(msg) < 12 {
		t.Fatalf("short DNS response: %d bytes", len(msg))
	}
	off := 12
	qd := int(binary.BigEndian.Uint16(msg[4:6]))
	for i := 0; i < qd; i++ {
		advance, err := skipQName(msg, off)
		if err != nil {
			t.Fatalf("skip QNAME: %v", err)
		}
		off = advance + 4 // QTYPE+QCLASS
	}
	ancount := int(binary.BigEndian.Uint16(msg[6:8]))
	var ips []net.IP
	for i := 0; i < ancount; i++ {
		advance, err := skipQName(msg, off)
		if err != nil {
			t.Fatalf("skip NAME: %v", err)
		}
		if advance+10 > len(msg) {
			return ips
		}
		rtype := binary.BigEndian.Uint16(msg[advance : advance+2])
		rdlen := int(binary.BigEndian.Uint16(msg[advance+8 : advance+10]))
		if advance+10+rdlen > len(msg) {
			return ips
		}
		if rtype == 1 && rdlen == 4 {
			ip := make(net.IP, 4)
			copy(ip, msg[advance+10:advance+14])
			ips = append(ips, ip)
		}
		off = advance + 10 + rdlen
	}
	return ips
}

// skipQName returns the offset just past the NAME field. Compression
// pointers (0xC0+) are one encoded jump, 2 bytes total.
func skipQName(msg []byte, off int) (int, error) {
	for off < len(msg) {
		b := msg[off]
		if b == 0 {
			return off + 1, nil
		}
		if b&0xC0 == 0xC0 {
			return off + 2, nil
		}
		off += 1 + int(b)
	}
	return 0, errors.New("unterminated NAME")
}
