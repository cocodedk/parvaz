// Package testutil provides an in-process stub of the Parvaz Cloudflare
// Worker. Tests use it to exercise the relay layer without hitting a
// real Cloudflare deployment.
package testutil

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"

	"github.com/coder/websocket"
)

// DialLog records one /tunnel hit — the client-supplied target.
type DialLog struct {
	Host string
	Port uint16
}

// WorkerStub is an in-process TLS httptest.Server that accepts WebSocket
// upgrades at /tunnel and by default echoes binary frames. Auth key is
// validated; mismatches return 401.
type WorkerStub struct {
	AuthKey string

	// UpstreamHandler, if non-nil, overrides the default echo. It owns
	// the conn lifecycle — the server side of a client-server net.Pipe
	// where the client side is wired to the WebSocket stream.
	UpstreamHandler func(host string, port uint16, conn io.ReadWriteCloser)

	mu     sync.Mutex
	Log    []DialLog
	server *httptest.Server
}

// NewWorkerStub starts a TLS httptest server that accepts WS upgrades.
func NewWorkerStub(authKey string) *WorkerStub {
	s := &WorkerStub{AuthKey: authKey}
	s.server = httptest.NewTLSServer(http.HandlerFunc(s.handle))
	return s
}

// HTTPClient returns a client that trusts the stub's test cert.
func (s *WorkerStub) HTTPClient() *http.Client { return s.server.Client() }

// WSURL returns the wss:// URL of the /tunnel endpoint.
func (s *WorkerStub) WSURL() string {
	return "wss://" + strings.TrimPrefix(s.server.URL, "https://") + "/tunnel"
}

// Close shuts down the stub.
func (s *WorkerStub) Close() { s.server.Close() }

// Hits returns the number of /tunnel requests recorded so far.
func (s *WorkerStub) Hits() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.Log)
}

func (s *WorkerStub) handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/tunnel" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	q := r.URL.Query()
	if q.Get("k") != s.AuthKey {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	host := q.Get("host")
	port64, err := strconv.ParseUint(q.Get("port"), 10, 16)
	if err != nil || host == "" || port64 == 0 {
		http.Error(w, "bad target", http.StatusBadRequest)
		return
	}
	port := uint16(port64)

	s.mu.Lock()
	s.Log = append(s.Log, DialLog{Host: host, Port: port})
	s.mu.Unlock()

	ws, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer ws.CloseNow()

	conn := websocket.NetConn(r.Context(), ws, websocket.MessageBinary)
	defer conn.Close()

	if s.UpstreamHandler != nil {
		s.UpstreamHandler(host, port, conn)
		return
	}
	// Default: echo. Reads from conn, writes back.
	_, _ = io.Copy(conn, conn)
}
