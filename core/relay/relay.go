// Package relay glues the protocol envelope, the fronted HTTP client, and
// the content codec into a single request-in / response-out facade.
//
// A Relay is the only layer that ties JSON + network + codec together; every
// other package stays single-responsibility.
package relay

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"

	"github.com/cocodedk/parvaz/core/codec"
	"github.com/cocodedk/parvaz/core/protocol"
)

// Config configures a Relay. All fields are required.
type Config struct {
	// HTTPClient sends fronted POSTs. Typically built with fronter.NewHTTPClient.
	HTTPClient *http.Client

	// ScriptURLs are Apps Script deployment URLs to POST envelopes to.
	// Multiple URLs are round-robined across calls.
	ScriptURLs []string

	// AuthKey is the shared secret with Code.gs.
	AuthKey string
}

// Relay sends protocol.Request through the fronted transport and returns
// the decoded protocol.Response.
type Relay struct {
	cfg  Config
	next atomic.Uint32
}

// New validates the config and returns a ready Relay.
func New(cfg Config) (*Relay, error) {
	if cfg.HTTPClient == nil {
		return nil, errors.New("relay: HTTPClient required")
	}
	if len(cfg.ScriptURLs) == 0 {
		return nil, errors.New("relay: at least one ScriptURL required")
	}
	if cfg.AuthKey == "" {
		return nil, errors.New("relay: AuthKey required")
	}
	return &Relay{cfg: cfg}, nil
}

// Do sends req through the relay and returns the decoded response. On
// transport or protocol errors, the returned error is wrapped for context;
// Apps-Script-level errors come back as *protocol.ServerError.
func (r *Relay) Do(ctx context.Context, req protocol.Request) (*protocol.Response, error) {
	body, err := protocol.EncodeSingle(req, r.cfg.AuthKey)
	if err != nil {
		return nil, fmt.Errorf("relay: encode: %w", err)
	}
	url := r.pickURL()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("relay: new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := r.cfg.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("relay: http: %w", err)
	}
	defer httpResp.Body.Close()

	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("relay: read body: %w", err)
	}

	resp, err := protocol.DecodeResponse(raw)
	if err != nil {
		return nil, err // already a typed error (ServerError or parse error)
	}

	if ce := resp.Header.Get("Content-Encoding"); ce != "" {
		decoded, err := codec.Decode(resp.Body, ce)
		if err != nil {
			return nil, fmt.Errorf("relay: decode body: %w", err)
		}
		resp.Body = decoded
		resp.Header.Del("Content-Encoding")
	}
	return resp, nil
}

// pickURL round-robins across the configured ScriptURLs.
func (r *Relay) pickURL() string {
	n := r.next.Add(1) - 1
	return r.cfg.ScriptURLs[int(n)%len(r.cfg.ScriptURLs)]
}
