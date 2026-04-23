package fronter

import (
	"context"
	"net"
	"net/http"
	"time"
)

// NewHTTPClient returns an *http.Client that routes every request through
// the fronter Dialer.
//
// Transport behavior:
//   - The addr parsed from req.URL.Host is IGNORED at the transport layer;
//     every TLS connection goes to the fixed `target` (e.g. "216.239.38.120:443").
//   - The HTTP Host: header is still taken from req.URL.Host by net/http,
//     so Google's edge routes internally once TLS terminates.
//   - TLS SNI is whatever d.FrontDomain is (the DPI-visible front).
//
// HTTP/2 is NOT disabled here; the current Dialer negotiates no ALPN by
// default, so the connection is HTTP/1.1 unless explicit h2 support is
// added in Milestone 7.
func NewHTTPClient(d *Dialer, target string) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialTLSContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return d.Dial(ctx, "tcp", target)
			},
			ResponseHeaderTimeout: 30 * time.Second,
			IdleConnTimeout:       90 * time.Second,
		},
	}
}
