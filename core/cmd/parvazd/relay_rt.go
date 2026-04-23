package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/cocodedk/parvaz/core/protocol"
)

// relayRoundTripper translates one http.Request into one relay.Relay.Do.
// Lets the DoH leg (or any future caller) pretend it has a normal
// http.Client while the actual bytes ride the Apps Script envelope —
// binary survives intact via the envelope's base64 encoding (see
// core/protocol/encode.go).
type relayRoundTripper struct{ relay Relayer }

func (t *relayRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	var body []byte
	if r.Body != nil {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, fmt.Errorf("relay rt: read body: %w", err)
		}
		body = b
		_ = r.Body.Close()
	}
	resp, err := t.relay.Do(r.Context(), protocol.Request{
		Method:      r.Method,
		URL:         r.URL.String(),
		Header:      r.Header.Clone(),
		Body:        body,
		ContentType: r.Header.Get("Content-Type"),
	})
	if err != nil {
		return nil, err
	}
	hdr := http.Header{}
	for k, vs := range resp.Header {
		hdr[k] = vs
	}
	return &http.Response{
		StatusCode: resp.Status,
		Status:     fmt.Sprintf("%d", resp.Status),
		Header:     hdr,
		Body:       io.NopCloser(bytes.NewReader(resp.Body)),
		Request:    r,
		ProtoMajor: 1, ProtoMinor: 1,
		Proto: "HTTP/1.1",
	}, nil
}
