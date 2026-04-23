package relay_test

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cocodedk/parvaz/core/fronter"
	"github.com/cocodedk/parvaz/core/protocol"
	"github.com/cocodedk/parvaz/core/relay"
)

// Live end-to-end smoke tests that hit a real deployed Apps Script.
// Skipped unless both env vars are set, so `go test ./...` without
// credentials is still clean. Running locally after deploying
// reference/apps_script/Code.gs (see scripts/e2e/DEPLOY_CODE_GS.md):
//
//	export PARVAZ_LIVE_DEPLOYMENT_ID=AKfycbxxxx...
//	export PARVAZ_LIVE_AUTH_KEY=yourRandomSecretHere
//	go test -C core -v -run TestRelay_Live ./relay/...
//
// These tests consume Apps Script quota (~20k fetches/day free tier) —
// keep them narrow. Each run issues exactly one fetch.

const (
	liveGoogleIP    = "216.239.38.120" // production front IP
	liveFrontDomain = "www.google.com" // production SNI
	liveTimeout     = 30 * time.Second
)

func liveEnv(t *testing.T) (deploymentID, authKey string) {
	t.Helper()
	deploymentID = os.Getenv("PARVAZ_LIVE_DEPLOYMENT_ID")
	authKey = os.Getenv("PARVAZ_LIVE_AUTH_KEY")
	if deploymentID == "" || authKey == "" {
		t.Skip("live test: set PARVAZ_LIVE_DEPLOYMENT_ID + PARVAZ_LIVE_AUTH_KEY to run")
	}
	return deploymentID, authKey
}

// liveRelay wires a real fronter dialer at the production Google edge
// and a relay Client pointing at the given Apps Script deployment.
func liveRelay(t *testing.T, authKey, scriptURL string) *relay.Relay {
	t.Helper()
	d := &fronter.Dialer{
		FrontDomain:      liveFrontDomain,
		DialTimeout:      liveTimeout,
		HandshakeTimeout: liveTimeout,
	}
	target := net.JoinHostPort(liveGoogleIP, "443")
	client := fronter.NewHTTPClient(d, target)
	r, err := relay.New(relay.Config{
		HTTPClient: client,
		ScriptURLs: []string{scriptURL},
		AuthKey:    authKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestRelay_Live_GetExampleCom(t *testing.T) {
	deploymentID, authKey := liveEnv(t)
	scriptURL := "https://script.google.com/macros/s/" + deploymentID + "/exec"
	r := liveRelay(t, authKey, scriptURL)

	ctx, cancel := context.WithTimeout(context.Background(), liveTimeout)
	defer cancel()

	resp, err := r.Do(ctx, protocol.Request{
		Method: "GET", URL: "https://example.com/", FollowRedirects: true,
	})
	if err != nil {
		t.Fatalf("live Do: %v", err)
	}
	if resp.Status != 200 {
		t.Errorf("status = %d, want 200", resp.Status)
	}
	// Example.com has served variants of this over the years; "example domain"
	// has remained stable as the title/body lede.
	if !bytes.Contains(bytes.ToLower(resp.Body), []byte("example domain")) {
		t.Errorf("body doesn't contain 'example domain'; got first 200 bytes: %q",
			firstNBytes(resp.Body, 200))
	}
}

func TestRelay_Live_UnauthorizedKeyRejected(t *testing.T) {
	deploymentID, _ := liveEnv(t)
	scriptURL := "https://script.google.com/macros/s/" + deploymentID + "/exec"
	r := liveRelay(t, "definitely-not-the-real-key", scriptURL)

	ctx, cancel := context.WithTimeout(context.Background(), liveTimeout)
	defer cancel()

	_, err := r.Do(ctx, protocol.Request{
		Method: "GET", URL: "https://example.com/", FollowRedirects: true,
	})
	if err == nil {
		t.Fatal("expected unauthorized error from bad key; got nil")
	}
	var srv *protocol.ServerError
	if !errors.As(err, &srv) || !strings.Contains(srv.Message, "unauthorized") {
		t.Errorf("err = %v, want *ServerError{unauthorized}", err)
	}
}

func firstNBytes(b []byte, n int) []byte {
	if len(b) < n {
		return b
	}
	return b[:n]
}
