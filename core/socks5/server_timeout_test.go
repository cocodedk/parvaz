package socks5

import (
	"net"
	"testing"
	"time"
)

func TestSOCKS5_HandshakeTimeout_ClosesSilentClient(t *testing.T) {
	// A client that connects but writes nothing must be closed by the server
	// within its HandshakeTimeout.
	srv := &Server{Dialer: &recordingDialer{}, HandshakeTimeout: 100 * time.Millisecond}
	addr, stop := startServerWith(t, srv)
	defer stop()

	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	_ = c.SetReadDeadline(time.Now().Add(1 * time.Second))
	start := time.Now()
	buf := make([]byte, 4)
	n, err := c.Read(buf)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatalf("expected EOF from server-side close, got %d bytes", n)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("server closed after %s, expected near HandshakeTimeout=100ms", elapsed)
	}
}
