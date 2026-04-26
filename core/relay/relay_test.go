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
