package socks5

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

// DatagramHandler routes a single UDP datagram arriving on the SOCKS5
// UDP ASSOCIATE port. Returns the response payload to send back to the
// client, or (nil, nil) to drop silently. Any non-nil error is logged
// at Debug and the datagram is dropped.
//
// The server owns the SOCKS5 UDP framing — the handler sees only the
// decoded destination + raw payload, and returns only the raw response
// payload. The server re-encodes the SOCKS5 UDP header with the
// original destination so xjasonlyu's symmetric-NAT check accepts it.
type DatagramHandler interface {
	Handle(ctx context.Context, dstHost string, dstPort uint16, payload []byte) ([]byte, error)
}

const (
	udpMaxDatagram    = 64 * 1024
	udpReadDeadline   = 500 * time.Millisecond
	udpWriteDeadline  = 5 * time.Second
	dgramHandleBudget = 10 * time.Second
)

// handleUDPAssociate fulfils RFC 1928 §4 CMD=0x03. The TCP control
// connection's lifetime bounds the association: while it stays open
// we accept datagrams on the bound UDP port; when it closes, we tear
// down the PacketConn. Per codex-review: we ignore the DST.ADDR /
// DST.PORT in the ASSOCIATE request — xjasonlyu and most clients
// send 0.0.0.0:0 and rely on the per-datagram header being
// authoritative.
func (s *Server) handleUDPAssociate(ctx context.Context, tcp net.Conn, atyp byte) error {
	if s.Datagram == nil {
		_, _ = tcp.Write(replyCmdNope)
		return errors.New("UDP ASSOCIATE requested but no DatagramHandler configured")
	}
	if _, err := readAddr(tcp, atyp); err != nil {
		_, _ = tcp.Write(replyAtypBad)
		return err
	}
	var suggestedPort [2]byte
	if _, err := io.ReadFull(tcp, suggestedPort[:]); err != nil {
		return err
	}

	pc, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		_, _ = tcp.Write(replyFail)
		return fmt.Errorf("listen udp: %w", err)
	}
	defer pc.Close()

	local := pc.LocalAddr().(*net.UDPAddr)
	if _, err := tcp.Write(buildAssociateReply(local)); err != nil {
		return err
	}
	_ = tcp.SetDeadline(time.Time{})

	// Any TCP read side closure tears down the association (RFC 1928 §6).
	assocCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		_, _ = io.Copy(io.Discard, tcp)
		cancel()
	}()

	s.pumpDatagrams(assocCtx, pc)
	return nil
}

func (s *Server) pumpDatagrams(ctx context.Context, pc net.PacketConn) {
	buf := make([]byte, udpMaxDatagram)
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		_ = pc.SetReadDeadline(time.Now().Add(udpReadDeadline))
		n, src, err := pc.ReadFrom(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			return
		}
		payload := make([]byte, n)
		copy(payload, buf[:n])
		go s.handleDatagram(ctx, pc, src, payload)
	}
}

func (s *Server) handleDatagram(ctx context.Context, pc net.PacketConn, src net.Addr, dgram []byte) {
	dstHost, dstPort, body, ok := decodeDatagram(dgram)
	if !ok {
		s.logger().Debug("socks5 udp: malformed datagram", "src", src.String(), "len", len(dgram))
		return
	}
	handleCtx, cancel := context.WithTimeout(ctx, dgramHandleBudget)
	defer cancel()
	resp, err := s.Datagram.Handle(handleCtx, dstHost, dstPort, body)
	if err != nil {
		s.logger().Debug("socks5 udp: handler error",
			"dst", net.JoinHostPort(dstHost, fmt.Sprint(dstPort)), "err", err)
		return
	}
	if resp == nil {
		return
	}
	reply, err := encodeDatagram(dstHost, dstPort, resp)
	if err != nil {
		s.logger().Debug("socks5 udp: encode reply", "err", err)
		return
	}
	_ = pc.SetWriteDeadline(time.Now().Add(udpWriteDeadline))
	_, _ = pc.WriteTo(reply, src)
}
