package relay_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/cocodedk/parvaz/core/internal/testutil"
	"github.com/cocodedk/parvaz/core/protocol"
)

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
