// Package relay opens raw TCP tunnels to the Parvaz Cloudflare Worker.
//
// Each SOCKS5 CONNECT becomes one WebSocket request:
//
//	GET /tunnel?k=<access_key>&host=<target>&port=<target_port>
//	Upgrade: websocket
//
// The Worker accepts, opens an upstream TCP socket via cloudflare:sockets,
// and pipes the WebSocket binary frames to/from that socket. Parvaz wraps
// the WebSocket back into a net.Conn so the rest of the pipeline sees
// a transparent TCP stream.
//
// This package implements socks5.Dialer — one Dial per tunnel.
package relay

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"

	"github.com/coder/websocket"
)

// Config configures a Relay. All fields are required.
type Config struct {
	// HTTPClient performs the WebSocket upgrade. In production this is
	// built via fronter.NewHTTPClient so SNI is split from Host.
	HTTPClient *http.Client

	// WorkerURL is the wss:// (or ws:// for tests) URL of the deployed
	// Cloudflare Worker's /tunnel endpoint. Path must be /tunnel.
	WorkerURL string

	// AuthKey is the shared secret with the Worker. Sent as `?k=` query param.
	AuthKey string
}

// Relay opens tunneled TCP streams. Goroutine-safe.
type Relay struct {
	cfg Config
}

// New validates config and returns a ready Relay.
func New(cfg Config) (*Relay, error) {
	if cfg.HTTPClient == nil {
		return nil, errors.New("relay: HTTPClient required")
	}
	if cfg.AuthKey == "" {
		return nil, errors.New("relay: AuthKey required")
	}
	u, err := url.Parse(cfg.WorkerURL)
	if err != nil {
		return nil, fmt.Errorf("relay: parse WorkerURL: %w", err)
	}
	if u.Scheme != "wss" && u.Scheme != "ws" {
		return nil, fmt.Errorf("relay: WorkerURL scheme must be ws/wss, got %q", u.Scheme)
	}
	if u.Host == "" {
		return nil, errors.New("relay: WorkerURL missing host")
	}
	return &Relay{cfg: cfg}, nil
}

// Dial opens a tunneled TCP connection to host:port via the Worker and
// returns the conn as a net.Conn. Implements socks5.Dialer.
func (r *Relay) Dial(ctx context.Context, host string, port uint16) (net.Conn, error) {
	if host == "" {
		return nil, errors.New("relay: empty host")
	}
	if port == 0 {
		return nil, errors.New("relay: zero port")
	}
	u, err := url.Parse(r.cfg.WorkerURL)
	if err != nil {
		return nil, fmt.Errorf("relay: parse worker url: %w", err)
	}
	q := u.Query()
	q.Set("k", r.cfg.AuthKey)
	q.Set("host", host)
	q.Set("port", fmt.Sprint(port))
	u.RawQuery = q.Encode()

	opts := &websocket.DialOptions{HTTPClient: r.cfg.HTTPClient}
	wsConn, resp, err := websocket.Dial(ctx, u.String(), opts)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("relay: unauthorized — check access key")
		}
		return nil, fmt.Errorf("relay: dial ws: %w", err)
	}

	// Detach the net.Conn lifetime from the dial context — callers control
	// the conn's lifetime via Close(). A background ctx keeps it open.
	return websocket.NetConn(context.Background(), wsConn, websocket.MessageBinary), nil
}
