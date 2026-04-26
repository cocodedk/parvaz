package relay_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/cocodedk/parvaz/core/fronter"
	"github.com/cocodedk/parvaz/core/internal/testutil"
	"github.com/cocodedk/parvaz/core/protocol"
	"github.com/cocodedk/parvaz/core/relay"
)

// newRelay wires stub → fronter → relay and returns it ready to use.
func newRelay(t *testing.T, stub *testutil.AppsScriptStub, scriptURLs []string, authKey string) *relay.Relay {
	t.Helper()
	u, err := url.Parse(stub.BaseURL())
	if err != nil {
		t.Fatal(err)
	}
	d := &fronter.Dialer{FrontDomain: "www.google.com", InsecureSkipVerify: true}
	client := fronter.NewHTTPClient(d, u.Host)
	r, err := relay.New(relay.Config{HTTPClient: client, ScriptURLs: scriptURLs, AuthKey: authKey})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestRelay_GET_TunnelsThroughStub(t *testing.T) {
	stub := testutil.NewStub("secret")
	defer stub.Close()
	stub.Routes["GET https://api.example.com/hi"] = testutil.StubResponse{
		Status: 200, Header: map[string]string{"Content-Type": "text/plain"},
		Body: []byte("hello from target"),
	}
	r := newRelay(t, stub, []string{stub.BaseURL() + "/macros/s/S1/exec"}, "secret")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := r.Do(ctx, protocol.Request{Method: "GET", URL: "https://api.example.com/hi", FollowRedirects: true})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.Status != 200 {
		t.Errorf("status = %d, want 200", resp.Status)
	}
	if !bytes.Equal(resp.Body, []byte("hello from target")) {
		t.Errorf("body = %q", resp.Body)
	}
}

func TestRelay_POST_BodyBase64RoundTrip(t *testing.T) {
	stub := testutil.NewStub("secret")
	defer stub.Close()
	stub.Routes["POST https://api.example.com/echo"] = testutil.StubResponse{
		Status: 201, Body: []byte("got it"),
	}
	r := newRelay(t, stub, []string{stub.BaseURL() + "/macros/s/S1/exec"}, "secret")

	payload := []byte(`{"from":"parvaz","n":42}`)
	ctx := context.Background()
	_, err := r.Do(ctx, protocol.Request{
		Method: "POST", URL: "https://api.example.com/echo",
		Body: payload, ContentType: "application/json", FollowRedirects: true,
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if len(stub.Log) != 1 {
		t.Fatalf("log len = %d, want 1", len(stub.Log))
	}
	got := stub.Log[0]
	if !bytes.Equal(got.Body, payload) {
		t.Errorf("server-side body = %q, want %q", got.Body, payload)
	}
	if got.ContentType != "application/json" {
		t.Errorf("ct = %q", got.ContentType)
	}
}

func TestRelay_HonorsContentEncoding(t *testing.T) {
	stub := testutil.NewStub("secret")
	defer stub.Close()
	raw := []byte("the fast brown fox jumps over the lazy dog" + string(bytes.Repeat([]byte(" x"), 40)))
	var gz bytes.Buffer
	gw := gzip.NewWriter(&gz)
	_, _ = gw.Write(raw)
	_ = gw.Close()
	stub.Routes["GET https://api.example.com/gz"] = testutil.StubResponse{
		Status: 200, Header: map[string]string{
			"Content-Type":   "text/plain",
			"Content-Length": strconv.Itoa(gz.Len()),
		},
		Body: gz.Bytes(), ContentEncoding: "gzip",
	}
	r := newRelay(t, stub, []string{stub.BaseURL() + "/macros/s/S1/exec"}, "secret")

	resp, err := r.Do(context.Background(), protocol.Request{
		Method: "GET", URL: "https://api.example.com/gz", FollowRedirects: true,
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if !bytes.Equal(resp.Body, raw) {
		t.Errorf("body not decompressed: got %q (len %d) want len %d", resp.Body, len(resp.Body), len(raw))
	}
	if resp.Header.Get("Content-Encoding") != "" {
		t.Errorf("Content-Encoding still set after decode: %q", resp.Header.Get("Content-Encoding"))
	}
	if resp.Header.Get("Content-Length") != "" {
		t.Errorf("stale Content-Length after decode: %q (was %d compressed, body is now %d)",
			resp.Header.Get("Content-Length"), gz.Len(), len(resp.Body))
	}
}

func TestRelay_UnauthorizedFromStub_ReturnsTypedError(t *testing.T) {
	stub := testutil.NewStub("secret")
	defer stub.Close()
	r := newRelay(t, stub, []string{stub.BaseURL() + "/macros/s/S1/exec"}, "wrong-key")

	_, err := r.Do(context.Background(), protocol.Request{
		Method: "GET", URL: "https://api.example.com/hi", FollowRedirects: true,
	})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
	var srv *protocol.ServerError
	if !errors.As(err, &srv) || srv.Message != "unauthorized" {
		t.Errorf("err = %v, want *ServerError{unauthorized}", err)
	}
}

// DoBatch is the runtime hook for the perf-throughput optimization.
// One batch envelope = one Apps Script invocation = one fixed cost
// (~300–1500 ms) amortized across N items via UrlFetchApp.fetchAll on
// the server side. The test asserts (a) order is preserved, (b) the
// stub saw exactly ONE envelope hit (not N), and (c) per-item bodies
// round-trip intact.
func TestRelay_DoBatch_RoundTripsAndCoalescesEnvelope(t *testing.T) {
	stub := testutil.NewStub("secret")
	defer stub.Close()
	stub.Routes["GET https://api.example.com/a"] = testutil.StubResponse{Status: 200, Body: []byte("AA")}
	stub.Routes["GET https://api.example.com/b"] = testutil.StubResponse{Status: 201, Body: []byte("BB")}
	stub.Routes["GET https://api.example.com/c"] = testutil.StubResponse{Status: 202, Body: []byte("CC")}
	r := newRelay(t, stub, []string{stub.BaseURL() + "/macros/s/S1/exec"}, "secret")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	bresp, err := r.DoBatch(ctx, protocol.BatchRequest{Items: []protocol.Request{
		{Method: "GET", URL: "https://api.example.com/a", FollowRedirects: true},
		{Method: "GET", URL: "https://api.example.com/b", FollowRedirects: true},
		{Method: "GET", URL: "https://api.example.com/c", FollowRedirects: true},
	}})
	if err != nil {
		t.Fatalf("DoBatch: %v", err)
	}
	if len(bresp.Items) != 3 {
		t.Fatalf("items = %d, want 3", len(bresp.Items))
	}
	wantBodies := [][]byte{[]byte("AA"), []byte("BB"), []byte("CC")}
	wantStatus := []int{200, 201, 202}
	for i, it := range bresp.Items {
		if it.Err != nil {
			t.Errorf("item %d err: %v", i, it.Err)
			continue
		}
		if it.Response.Status != wantStatus[i] {
			t.Errorf("item %d status = %d, want %d", i, it.Response.Status, wantStatus[i])
		}
		if !bytes.Equal(it.Response.Body, wantBodies[i]) {
			t.Errorf("item %d body = %q, want %q", i, it.Response.Body, wantBodies[i])
		}
	}
	if len(stub.Log) != 1 {
		t.Errorf("stub hit count = %d, want 1 (batch must be ONE envelope)", len(stub.Log))
	}
}

// Per-item server errors must reach only the affected caller, and
// successful items in the same batch must still return cleanly. This
// is what lets the Coalescer treat batches as best-effort across
// independent callers.
func TestRelay_DoBatch_PerItemErrorsIsolated(t *testing.T) {
	stub := testutil.NewStub("secret")
	defer stub.Close()
	stub.Routes["GET https://api.example.com/ok"] = testutil.StubResponse{Status: 200, Body: []byte("ok")}
	// Second item (bad URL scheme) — stub returns per-item {e:"bad url"} like Code.gs:64
	r := newRelay(t, stub, []string{stub.BaseURL() + "/macros/s/S1/exec"}, "secret")

	bresp, err := r.DoBatch(context.Background(), protocol.BatchRequest{Items: []protocol.Request{
		{Method: "GET", URL: "https://api.example.com/ok", FollowRedirects: true},
		{Method: "GET", URL: "ftp://nope/x", FollowRedirects: true},
	}})
	if err != nil {
		t.Fatalf("DoBatch returned top-level err: %v", err)
	}
	if len(bresp.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(bresp.Items))
	}
	if bresp.Items[0].Err != nil {
		t.Errorf("item 0 should be ok, got err=%v", bresp.Items[0].Err)
	}
	if bresp.Items[0].Response == nil || !bytes.Equal(bresp.Items[0].Response.Body, []byte("ok")) {
		t.Errorf("item 0 body = %v, want \"ok\"", bresp.Items[0].Response)
	}
	if bresp.Items[1].Err == nil {
		t.Error("item 1 expected per-item error, got nil")
	}
}

func TestRelay_MultipleScriptIDs_RoundRobins(t *testing.T) {
	stub := testutil.NewStub("secret")
	defer stub.Close()
	stub.Routes["GET https://api.example.com/ping"] = testutil.StubResponse{Status: 200, Body: []byte("pong")}
	urls := []string{
		stub.BaseURL() + "/macros/s/S1/exec",
		stub.BaseURL() + "/macros/s/S2/exec",
	}
	r := newRelay(t, stub, urls, "secret")

	for i := 0; i < 4; i++ {
		_, err := r.Do(context.Background(), protocol.Request{
			Method: "GET", URL: "https://api.example.com/ping", FollowRedirects: true,
		})
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
	}
	if len(stub.Log) != 4 {
		t.Fatalf("log len = %d, want 4", len(stub.Log))
	}
	wantPaths := []string{
		"/macros/s/S1/exec", "/macros/s/S2/exec",
		"/macros/s/S1/exec", "/macros/s/S2/exec",
	}
	for i, entry := range stub.Log {
		if entry.Path != wantPaths[i] {
			t.Errorf("hit[%d].path = %q, want %q", i, entry.Path, wantPaths[i])
		}
	}
}
