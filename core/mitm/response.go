package mitm

import (
	"bytes"
	"io"
	"net/http"

	"github.com/cocodedk/parvaz/core/protocol"
)

// writeResponse serializes a protocol.Response back onto the intercepted
// client connection as an HTTP/1.1 response. The Request is attached to
// resp so http.Response.Write picks the right framing (CONNECT vs. plain).
func writeResponse(w io.Writer, req *http.Request, pr *protocol.Response) error {
	status := pr.Status
	if status == 0 {
		status = 200
	}
	header := pr.Header
	if header == nil {
		header = http.Header{}
	}
	resp := &http.Response{
		Status:        http.StatusText(status),
		StatusCode:    status,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        header,
		Body:          io.NopCloser(bytes.NewReader(pr.Body)),
		ContentLength: int64(len(pr.Body)),
		Request:       req,
	}
	return resp.Write(w)
}

// writeBadGateway synthesises a 502 response when the relay fails.
// Browsers render this as a normal error page.
func writeBadGateway(w io.Writer, reason string) error {
	body := []byte("parvaz relay error: " + reason)
	resp := &http.Response{
		StatusCode:    http.StatusBadGateway,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        http.Header{"Content-Type": []string{"text/plain; charset=utf-8"}},
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
	}
	return resp.Write(w)
}
