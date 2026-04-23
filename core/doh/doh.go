// Package doh is a minimal DNS-over-HTTPS client used by the SOCKS5
// UDP ASSOCIATE path to answer DNS/53 queries without ever putting a
// UDP datagram on the wire. The caller supplies the http.Client —
// parvazd wires one on top of the fronter transport so DoH flows
// through the Google-edge IP with SNI=www.google.com and Host=
// dns.google, bypassing the Apps Script envelope entirely.
//
// RFC 8484 wire format: POST the DNS query bytes with Content-Type:
// application/dns-message, receive the DNS response bytes as the body.
//
// We deliberately avoid the JSON "/resolve" API — raw wire format
// lets us forward the exact query the browser/system resolver sent
// and return the exact answer it expects, with no translation.
package doh

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
)

// DefaultEndpoint is Google Public DNS's RFC 8484 endpoint. Reachable
// over the fronter without DNS: the Google edge routes by Host header.
const DefaultEndpoint = "https://dns.google/dns-query"

// Client sends DNS-over-HTTPS queries. Zero value is not usable —
// both HTTP and Endpoint are required.
type Client struct {
	HTTP     *http.Client
	Endpoint string
}

// Query forwards one DNS wire message and returns the wire response.
//
// RFC 8484 §4.1: "In order to maximize HTTP cache friendliness, DoH
// clients using media formats that include the ID field from the DNS
// message header, such as 'application/dns-message', SHOULD use a DNS
// ID of 0 in every DNS request." We do that on the way in and restore
// the caller's original ID on the way out so the caller's resolver
// still matches the response to its query.
func (c *Client) Query(ctx context.Context, wire []byte) ([]byte, error) {
	if len(wire) < 12 {
		return nil, fmt.Errorf("doh: query too short (%d bytes, need at least 12)", len(wire))
	}
	if c.HTTP == nil {
		return nil, fmt.Errorf("doh: HTTP client is nil")
	}
	endpoint := c.Endpoint
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}

	origID := [2]byte{wire[0], wire[1]}
	msg := make([]byte, len(wire))
	copy(msg, wire)
	msg[0], msg[1] = 0, 0

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(msg))
	if err != nil {
		return nil, fmt.Errorf("doh: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("doh: do: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("doh: status %d", resp.StatusCode)
	}
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("doh: read body: %w", err)
	}
	if len(out) < 12 {
		return nil, fmt.Errorf("doh: truncated response (%d bytes)", len(out))
	}
	out[0], out[1] = origID[0], origID[1]
	return out, nil
}
