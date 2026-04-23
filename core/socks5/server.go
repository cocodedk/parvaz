// Package socks5 implements a minimal SOCKS5 server: no authentication,
// CONNECT-only (no BIND, no UDP ASSOCIATE). The Dialer callback supplies
// the upstream TCP tunnel.
//
// Wire format — RFC 1928:
//
//	negotiate →  [VER=0x05, NMETHODS, METHODS...]
//	           ←  [VER=0x05, METHOD]            (0x00 no-auth, 0xFF no match)
//	request   →  [VER=0x05, CMD, RSV=0x00, ATYP, DST.ADDR, DST.PORT]
//	           ←  [VER=0x05, REP, RSV=0x00, ATYP, BND.ADDR, BND.PORT]
//
// REP codes used: 0x00 success, 0x01 general failure, 0x07 command not
// supported, 0x08 address type not supported.
package socks5

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
)

// Dialer opens the upstream tunnel for a CONNECT request.
type Dialer interface {
	Dial(ctx context.Context, host string, port uint16) (net.Conn, error)
}

// Server accepts SOCKS5 connections and bridges CONNECTs via Dialer.
type Server struct {
	Dialer Dialer
}

// Serve blocks, accepting connections from ln until ctx is cancelled or
// ln.Accept returns a permanent error.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go s.handle(ctx, conn)
	}
}

// ServeConn drives one already-accepted connection through handshake +
// request. Exposed for tests; production should call Serve instead.
func (s *Server) ServeConn(ctx context.Context, conn net.Conn) {
	s.handle(ctx, conn)
}

func (s *Server) handle(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	if err := negotiateNoAuth(conn); err != nil {
		return
	}
	_ = s.doRequest(ctx, conn)
}

func negotiateNoAuth(conn net.Conn) error {
	hdr := make([]byte, 2)
	if _, err := io.ReadFull(conn, hdr); err != nil {
		return err
	}
	if hdr[0] != 0x05 {
		return fmt.Errorf("unsupported SOCKS version %d", hdr[0])
	}
	if hdr[1] == 0 {
		return errors.New("no methods offered")
	}
	methods := make([]byte, int(hdr[1]))
	if _, err := io.ReadFull(conn, methods); err != nil {
		return err
	}
	accept := byte(0xFF)
	for _, m := range methods {
		if m == 0x00 {
			accept = 0x00
			break
		}
	}
	if _, err := conn.Write([]byte{0x05, accept}); err != nil {
		return err
	}
	if accept == 0xFF {
		return errors.New("no acceptable methods")
	}
	return nil
}

var (
	replyOK      = []byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}
	replyFail    = []byte{0x05, 0x01, 0x00, 0x01, 0, 0, 0, 0, 0, 0}
	replyCmdNope = []byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0}
	replyAtypBad = []byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0}
)

func (s *Server) doRequest(ctx context.Context, conn net.Conn) error {
	hdr := make([]byte, 4)
	if _, err := io.ReadFull(conn, hdr); err != nil {
		return err
	}
	if hdr[0] != 0x05 {
		return fmt.Errorf("unsupported SOCKS version %d", hdr[0])
	}
	cmd, atyp := hdr[1], hdr[3]
	if cmd != 0x01 { // we only support CONNECT; reject BIND + UDP
		_, _ = conn.Write(replyCmdNope)
		return fmt.Errorf("unsupported CMD %d", cmd)
	}
	host, err := readAddr(conn, atyp)
	if err != nil {
		_, _ = conn.Write(replyAtypBad)
		return err
	}
	portBuf := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBuf); err != nil {
		return err
	}
	port := binary.BigEndian.Uint16(portBuf)

	target, err := s.Dialer.Dial(ctx, host, port)
	if err != nil {
		_, _ = conn.Write(replyFail)
		return err
	}
	defer target.Close()

	if _, err := conn.Write(replyOK); err != nil {
		return err
	}
	errc := make(chan error, 2)
	go func() { _, err := io.Copy(target, conn); errc <- err }()
	go func() { _, err := io.Copy(conn, target); errc <- err }()
	<-errc
	return nil
}

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
