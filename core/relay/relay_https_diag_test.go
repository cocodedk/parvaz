package relay_test

// TestRelay_Live_HTTPS_Matrix is a diagnostic — exercises the full
// production relay against multiple HTTPS sites and prints, per site,
// what the round-trip actually returned. The goal is to reproduce
// "only example.com works" without a phone, by widening the range of
// sites and logging response shape (status, body size, body sniff,
// header keys, error type) so we can see WHERE the breakage is.
//
// Skipped unless PARVAZ_LIVE=1 + creds — same gating as relay_live_test.go.
//
//	source scripts/e2e/live.env; export PARVAZ_LIVE=1
//	go test -C core -v -run TestRelay_Live_HTTPS_Matrix ./relay/... -timeout 180s

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/cocodedk/parvaz/core/protocol"
)

func TestRelay_Live_HTTPS_Matrix(t *testing.T) {
	if os.Getenv("PARVAZ_LIVE") != "1" {
		t.Skip("set PARVAZ_LIVE=1 to run")
	}
	deploymentID, authKey := liveEnv(t)
	scriptURL := "https://script.google.com/macros/s/" + deploymentID + "/exec"
	r := liveRelay(t, authKey, scriptURL)

	cases := []struct {
		name string
		url  string
	}{
		{"example.com", "https://example.com/"},
		{"example.org", "https://example.org/"},
		{"www.google.com", "https://www.google.com/"},
		{"github.com", "https://github.com/"},
		{"en.wikipedia.org", "https://en.wikipedia.org/wiki/Main_Page"},
		{"cocode.dk", "https://cocode.dk/"},
		{"httpbin.org/get", "https://httpbin.org/get"},
	}

	type outcome struct {
		name     string
		status   int
		bodyLen  int
		bodyHead string
		headers  []string
		err      string
	}

	results := make([]outcome, 0, len(cases))
	for _, tc := range cases {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		resp, err := r.Do(ctx, protocol.Request{
			Method: "GET", URL: tc.url, FollowRedirects: true,
		})
		cancel()
		o := outcome{name: tc.name}
		if err != nil {
			o.err = fmt.Sprintf("%T: %v", err, err)
		} else {
			o.status = resp.Status
			o.bodyLen = len(resp.Body)
			head := resp.Body
			if len(head) > 200 {
				head = head[:200]
			}
			// Replace non-printable to keep table compact.
			cleaned := make([]byte, 0, len(head))
			for _, b := range head {
				if b == '\n' || b == '\r' || b == '\t' {
					cleaned = append(cleaned, ' ')
				} else if b < 32 || b > 126 {
					cleaned = append(cleaned, '.')
				} else {
					cleaned = append(cleaned, b)
				}
			}
			o.bodyHead = string(cleaned)
			keys := make([]string, 0, len(resp.Header))
			for k := range resp.Header {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			o.headers = keys
		}
		results = append(results, o)
	}

	t.Log("=== HTTPS matrix results ===")
	for _, o := range results {
		t.Logf("--- %s ---", o.name)
		if o.err != "" {
			t.Logf("    ERROR: %s", o.err)
			continue
		}
		t.Logf("    status=%d body=%d bytes", o.status, o.bodyLen)
		t.Logf("    headers=%s", strings.Join(o.headers, ", "))
		t.Logf("    body[0:200]=%s", o.bodyHead)
	}
}
