package socks5

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
)

// buildAssociateReply returns a SOCKS5 success reply with IP ATYP for
// the given bound loopback address. Codex-review specifically flagged
// domain ATYP as broken here because xjasonlyu's symmetric-NAT check
// compares the decoded UDPAddr against metadata.dst — domain names
// can't be resolved synchronously so the reply is dropped.
func buildAssociateReply(addr *net.UDPAddr) []byte {
	ip4 := addr.IP.To4()
	if ip4 == nil {
		ip4 = net.IPv4(127, 0, 0, 1).To4()
	}
	reply := []byte{0x05, 0x00, 0x00, 0x01, ip4[0], ip4[1], ip4[2], ip4[3], 0, 0}
	binary.BigEndian.PutUint16(reply[8:10], uint16(addr.Port))
	return reply
}

// decodeDatagram parses a SOCKS5 UDP request header + payload. Returns
// ok=false on any format violation (including FRAG != 0).
//
// Wire format (RFC 1928 §7):
//
//	+----+------+------+----------+----------+----------+
//	|RSV | FRAG | ATYP | DST.ADDR | DST.PORT |   DATA   |
//	+----+------+------+----------+----------+----------+
//	| 2  |  1   |  1   | variable |    2     | variable |
//	+----+------+------+----------+----------+----------+
func decodeDatagram(dgram []byte) (host string, port uint16, body []byte, ok bool) {
	if len(dgram) < 5 || dgram[0] != 0 || dgram[1] != 0 {
		return
	}
	if dgram[2] != 0 {
		return // fragmentation: drop per RFC + xjasonlyu behaviour
	}
	atyp := dgram[3]
	hostBytes, hostLen, err := decodeAddr(dgram[4:], atyp)
	if err != nil || len(dgram) < 4+hostLen+2 {
		return
	}
	port = binary.BigEndian.Uint16(dgram[4+hostLen : 4+hostLen+2])
	body = dgram[4+hostLen+2:]
	host = hostBytes
	ok = true
	return
}

func decodeAddr(b []byte, atyp byte) (string, int, error) {
	switch atyp {
	case 0x01:
		if len(b) < 4 {
			return "", 0, errors.New("short ipv4")
		}
		return net.IP(b[:4]).String(), 4, nil
	case 0x03:
		if len(b) < 1 {
			return "", 0, errors.New("short domain-len")
		}
		n := int(b[0])
		if len(b) < 1+n {
			return "", 0, errors.New("short domain")
		}
		return string(b[1 : 1+n]), 1 + n, nil
	case 0x04:
		if len(b) < 16 {
			return "", 0, errors.New("short ipv6")
		}
		return net.IP(b[:16]).String(), 16, nil
	default:
		return "", 0, fmt.Errorf("unsupported ATYP %d", atyp)
	}
}

// encodeDatagram builds a SOCKS5 UDP reply. IP ATYP preferred; falls
// back to domain ATYP when the host isn't a parseable IP literal.
func encodeDatagram(host string, port uint16, body []byte) ([]byte, error) {
	if ip, err := netip.ParseAddr(host); err == nil {
		if ip.Is4() {
			v4 := ip.As4()
			out := make([]byte, 0, 4+4+2+len(body))
			out = append(out, 0, 0, 0, 0x01)
			out = append(out, v4[:]...)
			out = append(out, byte(port>>8), byte(port))
			return append(out, body...), nil
		}
		v16 := ip.As16()
		out := make([]byte, 0, 4+16+2+len(body))
		out = append(out, 0, 0, 0, 0x04)
		out = append(out, v16[:]...)
		out = append(out, byte(port>>8), byte(port))
		return append(out, body...), nil
	}
	if len(host) > 255 {
		return nil, fmt.Errorf("domain too long (%d)", len(host))
	}
	out := make([]byte, 0, 4+1+len(host)+2+len(body))
	out = append(out, 0, 0, 0, 0x03, byte(len(host)))
	out = append(out, host...)
	out = append(out, byte(port>>8), byte(port))
	return append(out, body...), nil
}
