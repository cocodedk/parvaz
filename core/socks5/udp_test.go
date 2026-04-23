package socks5

import (
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"
)

// stubDatagramHandler captures every datagram and echoes a reply body.
type stubDatagramHandler struct {
	mu     sync.Mutex
	seen   [][]byte
	dstHost string
	dstPort uint16
	reply   []byte
}

func (h *stubDatagramHandler) Handle(ctx context.Context, host string, port uint16, payload []byte) ([]byte, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.dstHost, h.dstPort = host, port
	h.seen = append(h.seen, append([]byte(nil), payload...))
	return h.reply, nil
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// TestUDPAssociate_DNSRoundTrip covers the full flow a xjasonlyu client
// would run: TCP negotiate + UDP ASSOCIATE, then a UDP datagram to the
// returned bind addr, then reading a reply on the same socket.
func TestUDPAssociate_DNSRoundTrip(t *testing.T) {
	handler := &stubDatagramHandler{reply: []byte("dns-response")}
	srv := &Server{Datagram: handler, Logger: silentLogger()}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Serve(ctx, ln)

	tcp, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer tcp.Close()
	_ = tcp.SetDeadline(time.Now().Add(3 * time.Second))

	// Negotiate no-auth.
	if _, err := tcp.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		t.Fatalf("negotiate write: %v", err)
	}
	negResp := make([]byte, 2)
	if _, err := io.ReadFull(tcp, negResp); err != nil {
		t.Fatalf("negotiate read: %v", err)
	}

	// UDP ASSOCIATE with DST=0.0.0.0:0 (xjasonlyu-style).
	req := []byte{0x05, 0x03, 0x00, 0x01, 0, 0, 0, 0, 0, 0}
	if _, err := tcp.Write(req); err != nil {
		t.Fatalf("associate write: %v", err)
	}
	reply := make([]byte, 10)
	if _, err := io.ReadFull(tcp, reply); err != nil {
		t.Fatalf("associate read: %v", err)
	}
	if reply[0] != 0x05 || reply[1] != 0x00 || reply[3] != 0x01 {
		t.Fatalf("associate reply malformed: %v", reply)
	}
	bindIP := net.IPv4(reply[4], reply[5], reply[6], reply[7])
	bindPort := binary.BigEndian.Uint16(reply[8:10])
	if !bindIP.IsLoopback() {
		t.Errorf("bind IP not loopback: %v", bindIP)
	}

	// Send a DNS-shaped datagram.
	udp, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: bindIP, Port: int(bindPort)})
	if err != nil {
		t.Fatalf("dial udp: %v", err)
	}
	defer udp.Close()

	// SOCKS5 UDP header: RSV(2)+FRAG(1)+ATYP=0x01+IPv4(4)+Port(2).
	// Destination 10.0.0.2:53 — Parvaz's synthetic DNS.
	dgram := []byte{0, 0, 0, 0x01, 10, 0, 0, 2, 0, 53}
	dgram = append(dgram, []byte("dns-wire-bytes")...)
	if _, err := udp.Write(dgram); err != nil {
		t.Fatalf("udp write: %v", err)
	}
	_ = udp.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 1500)
	n, err := udp.Read(buf)
	if err != nil {
		t.Fatalf("udp read: %v", err)
	}

	// Reply should carry ORIGINAL dst in the SOCKS5 UDP header (codex
	// rule — xjasonlyu's symmetric-NAT wrapper compares the decoded
	// UDPAddr against the metadata destination).
	got := buf[:n]
	host, port, body, ok := decodeDatagram(got)
	if !ok {
		t.Fatalf("reply malformed: %v", got)
	}
	if host != "10.0.0.2" || port != 53 {
		t.Errorf("reply dst = %s:%d, want 10.0.0.2:53", host, port)
	}
	if string(body) != "dns-response" {
		t.Errorf("reply body = %q, want dns-response", body)
	}

	handler.mu.Lock()
	defer handler.mu.Unlock()
	if got, want := handler.dstHost, "10.0.0.2"; got != want {
		t.Errorf("handler saw dstHost = %q, want %q", got, want)
	}
	if handler.dstPort != 53 {
		t.Errorf("handler saw dstPort = %d, want 53", handler.dstPort)
	}
	if len(handler.seen) != 1 || string(handler.seen[0]) != "dns-wire-bytes" {
		t.Errorf("handler saw payloads = %v, want [dns-wire-bytes]", handler.seen)
	}
}

func TestUDPAssociate_RejectsFragmented(t *testing.T) {
	host, port, body, ok := decodeDatagram([]byte{0, 0, 0x01 /*FRAG*/, 0x01, 127, 0, 0, 1, 0, 53, 'x'})
	if ok {
		t.Errorf("fragmented datagram accepted: host=%s port=%d body=%q", host, port, body)
	}
}

func TestUDPAssociate_ReturnsCmdNopeWhenHandlerAbsent(t *testing.T) {
	srv := &Server{Logger: silentLogger()}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Serve(ctx, ln)

	tcp, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer tcp.Close()
	_ = tcp.SetDeadline(time.Now().Add(3 * time.Second))
	_, _ = tcp.Write([]byte{0x05, 0x01, 0x00})
	neg := make([]byte, 2)
	_, _ = io.ReadFull(tcp, neg)
	_, _ = tcp.Write([]byte{0x05, 0x03, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	reply := make([]byte, 10)
	_, _ = io.ReadFull(tcp, reply)
	if reply[1] != 0x07 {
		t.Errorf("expected REP=0x07 (command not supported), got 0x%02x", reply[1])
	}
}

func TestEncodeDecode_DatagramRoundTrip(t *testing.T) {
	cases := []struct {
		name, host string
		port       uint16
		body       []byte
	}{
		{"ipv4", "10.0.0.2", 53, []byte("hello")},
		{"ipv6", "2001:4860:4860::8888", 443, []byte("ping")},
		{"domain", "example.com", 80, []byte{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			encoded, err := encodeDatagram(c.host, c.port, c.body)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			host, port, body, ok := decodeDatagram(encoded)
			if !ok {
				t.Fatalf("decode rejected: %v", encoded)
			}
			if host != c.host || port != c.port || !reflect.DeepEqual(body, c.body) {
				t.Errorf("round-trip mismatch: got %s:%d %q, want %s:%d %q",
					host, port, body, c.host, c.port, c.body)
			}
		})
	}
}
