package socks5

import (
	"fmt"
	"io"
	"net"
)

// readAddr parses a SOCKS5 address per RFC 1928 §4, selecting the length
// encoding from the caller-provided ATYP byte. Returns the host portion only;
// the caller reads the two-byte port.
func readAddr(conn net.Conn, atyp byte) (string, error) {
	switch atyp {
	case 0x01:
		b := make([]byte, 4)
		if _, err := io.ReadFull(conn, b); err != nil {
			return "", err
		}
		return net.IP(b).String(), nil
	case 0x03:
		lenByte := make([]byte, 1)
		if _, err := io.ReadFull(conn, lenByte); err != nil {
			return "", err
		}
		b := make([]byte, int(lenByte[0]))
		if _, err := io.ReadFull(conn, b); err != nil {
			return "", err
		}
		return string(b), nil
	case 0x04:
		b := make([]byte, 16)
		if _, err := io.ReadFull(conn, b); err != nil {
			return "", err
		}
		return net.IP(b).String(), nil
	default:
		return "", fmt.Errorf("unsupported ATYP %d", atyp)
	}
}
