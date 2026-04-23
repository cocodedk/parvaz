package mitm

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/cocodedk/parvaz/core/protocol"
)

func TestInterceptor_GzipResponse_EchoedIntact(t *testing.T) {
	ca, err := LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// The relay is responsible for decoding Content-Encoding before handing
	// the response back; the interceptor should pass the decoded body through
	// to the browser as-is. Use a large, compressible payload to make sure
	// there's no accidental re-encode or truncation.
	payload := []byte(strings.Repeat("parvaz-flies-high ", 500))
	rec := &stubRelay{
		response: &protocol.Response{
			Status: 200,
			Header: http.Header{"Content-Type": []string{"text/plain"}},
			Body:   payload,
		},
	}
	ic := &Interceptor{CA: ca, Relay: rec}

	tlsClient, done := dialTLSToInterceptor(t, ic, "example.com", 443)
	defer func() {
		_ = tlsClient.Close()
		<-done
	}()

	req, err := http.NewRequest("GET", "https://example.com/large", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Accept-Encoding", "gzip") // realistic browser request
	if err := req.Write(tlsClient); err != nil {
		t.Fatalf("write req: %v", err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(tlsClient), req)
	if err != nil {
		t.Fatalf("read resp: %v", err)
	}
	defer resp.Body.Close()

	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("body length %d, want %d", len(got), len(payload))
	}
	if resp.Header.Get("Content-Encoding") != "" {
		t.Errorf("Content-Encoding leaked to client: %q", resp.Header.Get("Content-Encoding"))
	}
	// Compressibility sanity: the payload is repetitive; a naive gzip of
	// the received bytes should be smaller than what the client read, proving
	// the wire carried plain bytes.
	var gz bytes.Buffer
	gw := gzip.NewWriter(&gz)
	_, _ = gw.Write(payload)
	_ = gw.Close()
	if len(got) <= gz.Len() {
		t.Errorf("response size %d ≤ gzipped size %d (suspect it was compressed)", len(got), gz.Len())
	}
}
