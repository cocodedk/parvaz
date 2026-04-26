package relay_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cocodedk/parvaz/core/internal/testutil"
	"github.com/cocodedk/parvaz/core/protocol"
	"github.com/cocodedk/parvaz/core/relay"
)

// Coalescer with MaxBatch=1 degenerates to single-mode behavior:
// every submission flushes alone, but on the wire it's still a batch
// envelope (so the BATCH log entry counts each one).
func TestCoalescer_MaxBatchOne_FlushesImmediately(t *testing.T) {
	stub := testutil.NewStub("secret")
	defer stub.Close()
	stub.Routes["GET https://api.example.com/x"] = testutil.StubResponse{Status: 200, Body: []byte("X")}
	r := newRelay(t, stub, []string{stub.BaseURL() + "/macros/s/S1/exec"}, "secret")
	c := relay.NewCoalescer(r, relay.CoalescerConfig{Window: 50 * time.Millisecond, MaxBatch: 1})
	defer c.Close()

	resp, err := c.Do(context.Background(), protocol.Request{Method: "GET", URL: "https://api.example.com/x", FollowRedirects: true})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if resp.Status != 200 || string(resp.Body) != "X" {
		t.Errorf("resp = %+v", resp)
	}
}

// The whole point of the Coalescer: N concurrent submissions inside
// the window collapse into ONE Apps Script invocation.
func TestCoalescer_ConcurrentSubmissions_CollapseToOneEnvelope(t *testing.T) {
	stub := testutil.NewStub("secret")
	defer stub.Close()
	stub.Routes["GET https://api.example.com/a"] = testutil.StubResponse{Status: 200, Body: []byte("AA")}
	stub.Routes["GET https://api.example.com/b"] = testutil.StubResponse{Status: 200, Body: []byte("BB")}
	stub.Routes["GET https://api.example.com/c"] = testutil.StubResponse{Status: 200, Body: []byte("CC")}
	r := newRelay(t, stub, []string{stub.BaseURL() + "/macros/s/S1/exec"}, "secret")
	c := relay.NewCoalescer(r, relay.CoalescerConfig{Window: 100 * time.Millisecond, MaxBatch: 8})
	defer c.Close()

	urls := []string{
		"https://api.example.com/a",
		"https://api.example.com/b",
		"https://api.example.com/c",
	}
	results := make([]*protocol.Response, len(urls))
	errs := make([]error, len(urls))
	var wg sync.WaitGroup
	for i, u := range urls {
		wg.Add(1)
		go func(i int, u string) {
			defer wg.Done()
			results[i], errs[i] = c.Do(context.Background(),
				protocol.Request{Method: "GET", URL: u, FollowRedirects: true})
		}(i, u)
	}
	wg.Wait()

	for i, e := range errs {
		if e != nil {
			t.Errorf("submit %d: %v", i, e)
		}
	}
	if len(stub.Log) != 1 {
		t.Fatalf("stub hits = %d, want 1 (whole point of coalescing)", len(stub.Log))
	}
	if stub.Log[0].Method != "BATCH" {
		t.Errorf("log[0].Method = %q, want BATCH", stub.Log[0].Method)
	}
}

// Window expiry must flush even if MaxBatch wasn't reached. Submit one
// request with a short window — it must complete in roughly Window time,
// not block forever.
func TestCoalescer_WindowExpiryFlushesPending(t *testing.T) {
	stub := testutil.NewStub("secret")
	defer stub.Close()
	stub.Routes["GET https://api.example.com/x"] = testutil.StubResponse{Status: 200, Body: []byte("X")}
	r := newRelay(t, stub, []string{stub.BaseURL() + "/macros/s/S1/exec"}, "secret")
	c := relay.NewCoalescer(r, relay.CoalescerConfig{Window: 20 * time.Millisecond, MaxBatch: 8})
	defer c.Close()

	start := time.Now()
	resp, err := c.Do(context.Background(),
		protocol.Request{Method: "GET", URL: "https://api.example.com/x", FollowRedirects: true})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if resp.Status != 200 {
		t.Errorf("status = %d", resp.Status)
	}
	// Roughly window-bounded: 20ms..2s. Generous upper bound for slow CI.
	if elapsed < 15*time.Millisecond {
		t.Errorf("flush too eager: %s (window was 20ms)", elapsed)
	}
	if elapsed > 2*time.Second {
		t.Errorf("flush too slow: %s", elapsed)
	}
}

// MaxBatch forces an early flush before the window elapses. Submit
// MaxBatch concurrent items with a generous window — they should
// flush as soon as the cap is reached, not after Window.
func TestCoalescer_MaxBatchTriggersEarlyFlush(t *testing.T) {
	stub := testutil.NewStub("secret")
	defer stub.Close()
	for i, u := range []string{
		"https://api.example.com/a", "https://api.example.com/b",
		"https://api.example.com/c", "https://api.example.com/d",
	} {
		_ = i
		stub.Routes["GET "+u] = testutil.StubResponse{Status: 200, Body: []byte("ok")}
	}
	r := newRelay(t, stub, []string{stub.BaseURL() + "/macros/s/S1/exec"}, "secret")
	c := relay.NewCoalescer(r, relay.CoalescerConfig{Window: 5 * time.Second, MaxBatch: 4})
	defer c.Close()

	urls := []string{
		"https://api.example.com/a", "https://api.example.com/b",
		"https://api.example.com/c", "https://api.example.com/d",
	}
	start := time.Now()
	var wg sync.WaitGroup
	for _, u := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			_, _ = c.Do(context.Background(),
				protocol.Request{Method: "GET", URL: u, FollowRedirects: true})
		}(u)
	}
	wg.Wait()
	elapsed := time.Since(start)
	// Should NOT wait the full 5s window.
	if elapsed > 1*time.Second {
		t.Errorf("MaxBatch=4 didn't trigger early flush: elapsed %s (Window=5s)", elapsed)
	}
	if len(stub.Log) != 1 {
		t.Errorf("stub hits = %d, want 1", len(stub.Log))
	}
}

// A caller whose ctx is already cancelled when it tries to Submit
// must not block — and other callers in the same batch must still
// succeed. The cancelled caller gets ctx.Err(), the rest get their
// responses.
func TestCoalescer_CallerCtxCancel_DoesNotPoisonOthers(t *testing.T) {
	stub := testutil.NewStub("secret")
	defer stub.Close()
	stub.Routes["GET https://api.example.com/ok"] = testutil.StubResponse{Status: 200, Body: []byte("ok")}
	r := newRelay(t, stub, []string{stub.BaseURL() + "/macros/s/S1/exec"}, "secret")
	c := relay.NewCoalescer(r, relay.CoalescerConfig{Window: 50 * time.Millisecond, MaxBatch: 8})
	defer c.Close()

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.Do(cancelledCtx, protocol.Request{Method: "GET", URL: "https://api.example.com/ok", FollowRedirects: true})
	if err == nil {
		t.Error("cancelled-ctx Submit should error, got nil")
	}

	// A subsequent live submission must succeed.
	resp, err := c.Do(context.Background(),
		protocol.Request{Method: "GET", URL: "https://api.example.com/ok", FollowRedirects: true})
	if err != nil {
		t.Fatalf("live Submit: %v", err)
	}
	if string(resp.Body) != "ok" {
		t.Errorf("body = %q", resp.Body)
	}
}
