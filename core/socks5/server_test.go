package socks5

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// recordingDialer captures the Dial target; returns an immediately-closed
// pipe so the server's io.Copy loop exits cleanly.
type recordingDialer struct {
	mu   sync.Mutex
	Host string
	Port uint16
	Fail bool
	hits atomic.Int32
}

func (d *recordingDialer) Dial(_ context.Context, host string, port uint16) (net.Conn, error) {
	d.mu.Lock()
	d.Host, d.Port = host, port
	d.mu.Unlock()
	d.hits.Add(1)
	if d.Fail {
		return nil, errors.New("recording dialer: forced failure")
	}
	c1, c2 := net.Pipe()
	_ = c2.Close()
	return c1, nil
}

// startServer listens on 127.0.0.1:0 and runs a SOCKS5 server. Returns the
// listener address and a cleanup func.
func startServer(t *testing.T, d Dialer) (string, func()) {
	t.Helper()
	srv := &Server{Dialer: d}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = srv.Serve(ctx, ln) }()
	return ln.Addr().String(), func() {
		cancel()
		_ = ln.Close()
	}
}

func dialClient(t *testing.T, addr string) net.Conn {
	t.Helper()
	c, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_ = c.SetDeadline(time.Now().Add(2 * time.Second))
	return c
}

func TestSOCKS5_NoAuth_Negotiation(t *testing.T) {
	addr, stop := startServer(t, &recordingDialer{})
	defer stop()
	c := dialClient(t, addr)
	defer c.Close()

	// [VER=0x05, NMETHODS=1, METHODS={0x00}]
	if _, err := c.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, 2)
	if _, err := io.ReadFull(c, reply); err != nil {
		t.Fatal(err)
	}
	if reply[0] != 0x05 || reply[1] != 0x00 {
		t.Errorf("negotiation reply = %v, want [0x05, 0x00]", reply)
	}
}

func TestSOCKS5_CONNECT_ForwardsThroughRelay(t *testing.T) {
	d := &recordingDialer{}
	addr, stop := startServer(t, d)
	defer stop()
	c := dialClient(t, addr)
	defer c.Close()

	_, _ = c.Write([]byte{0x05, 0x01, 0x00})
	_, _ = io.ReadFull(c, make([]byte, 2))

	// CONNECT example.com:80, ATYP=domain
	req := []byte{0x05, 0x01, 0x00, 0x03, 11}
	req = append(req, "example.com"...)
	req = append(req, 0x00, 0x50) // port 80 big-endian
	if _, err := c.Write(req); err != nil {
		t.Fatal(err)
	}
	resp := make([]byte, 10)
	if _, err := io.ReadFull(c, resp); err != nil {
		t.Fatalf("read reply: %v", err)
	}
	if resp[0] != 0x05 || resp[1] != 0x00 {
		t.Errorf("CONNECT reply = %v, want VER=5 REP=0", resp)
	}
	d.mu.Lock()
	host, port := d.Host, d.Port
	d.mu.Unlock()
	if host != "example.com" || port != 80 {
		t.Errorf("dialer got %s:%d, want example.com:80", host, port)
	}
	if d.hits.Load() != 1 {
		t.Errorf("dialer hits = %d, want 1", d.hits.Load())
	}
}

func TestSOCKS5_RejectsBIND(t *testing.T) {
	addr, stop := startServer(t, &recordingDialer{})
	defer stop()
	c := dialClient(t, addr)
	defer c.Close()

	_, _ = c.Write([]byte{0x05, 0x01, 0x00})
	_, _ = io.ReadFull(c, make([]byte, 2))

	// CMD=0x02 (BIND), ATYP=IPv4, ADDR=1.2.3.4, PORT=80
	_, _ = c.Write([]byte{0x05, 0x02, 0x00, 0x01, 1, 2, 3, 4, 0x00, 0x50})
	resp := make([]byte, 10)
	if _, err := io.ReadFull(c, resp); err != nil {
		t.Fatal(err)
	}
	if resp[1] != 0x07 {
		t.Errorf("BIND REP = %d, want 0x07", resp[1])
	}
}

func TestSOCKS5_RejectsUDPAssociate(t *testing.T) {
	addr, stop := startServer(t, &recordingDialer{})
	defer stop()
	c := dialClient(t, addr)
	defer c.Close()

	_, _ = c.Write([]byte{0x05, 0x01, 0x00})
	_, _ = io.ReadFull(c, make([]byte, 2))

	// CMD=0x03 (UDP ASSOCIATE)
	_, _ = c.Write([]byte{0x05, 0x03, 0x00, 0x01, 0, 0, 0, 0, 0x00, 0x00})
	resp := make([]byte, 10)
	if _, err := io.ReadFull(c, resp); err != nil {
		t.Fatal(err)
	}
	if resp[1] != 0x07 {
		t.Errorf("UDP REP = %d, want 0x07", resp[1])
	}
}

func TestSOCKS5_MalformedHandshake_ClosesConn(t *testing.T) {
	addr, stop := startServer(t, &recordingDialer{})
	defer stop()
	c := dialClient(t, addr)
	defer c.Close()

	// Wrong VER
	if _, err := c.Write([]byte{0xAB, 0x01, 0x00}); err != nil {
		t.Fatal(err)
	}
	// Server should hang up without replying; read should EOF or timeout.
	_ = c.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	buf := make([]byte, 4)
	n, err := c.Read(buf)
	if err == nil {
		t.Errorf("expected EOF/timeout, got %d bytes: %v", n, buf[:n])
	}
}
