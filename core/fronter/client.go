package fronter

import (
	"context"
	"net"
	"net/http"
	"time"
)

// Pool sizing for the fronted leg. All fronted POSTs target a single
// Google edge IP, so net/http's default MaxIdleConnsPerHost = 2 is the
// effective concurrency cap and head-of-line-blocks every request past
// the second. We size for browser-like fan-out: a typical page load is
// dozens of small requests with bursty parallelism. 32 in-flight conns
// per host is generous without leaking fds.
const (
	defaultMaxIdleConnsPerHost = 16
	defaultMaxConnsPerHost     = 32
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
			MaxIdleConnsPerHost:   defaultMaxIdleConnsPerHost,
			MaxConnsPerHost:       defaultMaxConnsPerHost,
			ResponseHeaderTimeout: 30 * time.Second,
			IdleConnTimeout:       90 * time.Second,
		},
	}
}
