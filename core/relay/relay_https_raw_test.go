package relay_test

// TestRelay_Live_RawDump prints the raw HTTP response from Apps Script
// when given our envelope. If we're getting HTML back instead of JSON,
// the deployment is mis-configured (auth lapsed, permissions, redeploy).

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/cocodedk/parvaz/core/fronter"
	"github.com/cocodedk/parvaz/core/protocol"
)

func TestRelay_Live_RawDump(t *testing.T) {
	if os.Getenv("PARVAZ_LIVE") != "1" {
		t.Skip("set PARVAZ_LIVE=1 to run")
	}
	deploymentID, authKey := liveEnv(t)
	scriptURL := "https://script.google.com/macros/s/" + deploymentID + "/exec"

	d := &fronter.Dialer{
		FrontDomain:      liveFrontDomain,
		DialTimeout:      liveTimeout,
		HandshakeTimeout: liveTimeout,
	}
	target := net.JoinHostPort(liveGoogleIP, "443")
	client := fronter.NewHTTPClient(d, target)

	body, err := protocol.EncodeSingle(protocol.Request{
		Method: "GET", URL: "https://example.com/", FollowRedirects: true,
	}, authKey)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), liveTimeout)
	defer cancel()

	u, _ := url.Parse(scriptURL)
	t.Logf("POST %s  (host=%s)", scriptURL, u.Host)
	t.Logf("envelope: %d bytes", len(body))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, scriptURL, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("http: %v", err)
	}
	defer resp.Body.Close()

	t.Logf("=== response status: %s ===", resp.Status)
	keys := make([]string, 0, len(resp.Header))
	for k := range resp.Header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t.Logf("  %s: %s", k, strings.Join(resp.Header.Values(k), " | "))
	}

	buf := make([]byte, 4096)
	n, _ := resp.Body.Read(buf)
	t.Logf("=== first %d bytes of body ===", n)
	t.Log(string(buf[:n]))
}
