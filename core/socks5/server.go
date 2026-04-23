// Package socks5 implements a minimal SOCKS5 server for parvazd:
//   - CONNECT (CMD=0x01): Dialer callback supplies the upstream TCP tunnel.
//   - UDP ASSOCIATE (CMD=0x03): optional via DatagramHandler (see udp.go).
//     Used by tun2socks to forward DNS to the parvazd DoH shim.
//   - BIND: unsupported — replies 0x07.
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
	"log/slog"
	"net"
	"time"
)

// Dialer opens the upstream tunnel for a CONNECT request.
type Dialer interface {
	Dial(ctx context.Context, host string, port uint16) (net.Conn, error)
}

// Server accepts SOCKS5 connections and bridges CONNECTs via Dialer.
type Server struct {
	Dialer Dialer

	// Datagram handles UDP ASSOCIATE traffic. Nil → UDP ASSOCIATE is
	// rejected with REP=0x07 (command not supported), preserving the
	// pre-M15b-beta behaviour for callers that don't need UDP.
	Datagram DatagramHandler

	// Logger — nil uses slog.Default(). Failures log at Debug level.
	Logger *slog.Logger

	// HandshakeTimeout bounds the SOCKS negotiation + CONNECT request phase.
	// Cleared before the tunneled copy loop, which may be long-lived.
	// Zero means default (30s). Use a negative value to disable entirely.
	HandshakeTimeout time.Duration
}

const defaultHandshakeTimeout = 30 * time.Second

func (s *Server) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

func (s *Server) handshakeDeadline() time.Duration {
	switch {
	case s.HandshakeTimeout < 0:
		return 0
	case s.HandshakeTimeout == 0:
		return defaultHandshakeTimeout
	default:
		return s.HandshakeTimeout
	}
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
	if d := s.handshakeDeadline(); d > 0 {
		_ = conn.SetDeadline(time.Now().Add(d))
	}
	if err := negotiateNoAuth(conn); err != nil {
		s.logger().Debug("socks5: negotiate failed",
			"remote", conn.RemoteAddr().String(), "err", err)
		return
	}
	if err := s.doRequest(ctx, conn); err != nil {
		s.logger().Debug("socks5: request failed",
			"remote", conn.RemoteAddr().String(), "err", err)
	}
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
	switch cmd {
	case 0x01:
		// fall through to the CONNECT path below
	case 0x03:
		return s.handleUDPAssociate(ctx, conn, atyp)
	default:
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
	_ = conn.SetDeadline(time.Time{}) // tunnel may be long-lived
	errc := make(chan error, 2)
	go func() { _, err := io.Copy(target, conn); errc <- err }()
	go func() { _, err := io.Copy(conn, target); errc <- err }()
	<-errc
	return nil
}

