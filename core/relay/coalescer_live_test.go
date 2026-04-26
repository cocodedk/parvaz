package relay_test

// Live tests for the perf-throughput Phase 2 work — Relay.DoBatch and
// the Coalescer measured against a real deployed Apps Script. Skipped
// unless PARVAZ_LIVE=1 + PARVAZ_LIVE_DEPLOYMENT_ID + PARVAZ_LIVE_AUTH_KEY
// are set:
//
//	source scripts/e2e/live.env; export PARVAZ_LIVE=1
//	go test -C core -v -run TestRelay_Live_DoBatch ./relay/... -timeout 180s
//	go test -C core -v -run TestCoalescer_Live    ./relay/... -timeout 180s
//
// Quota note: TestRelay_Live_DoBatch consumes 1 Apps Script invocation
// (3 inner UrlFetchApp.fetchAll fetches). TestCoalescer_Live_AmortizesLatency
// consumes N + 1 invocations (N singles for the baseline, 1 batched).

import (
	"bytes"
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/cocodedk/parvaz/core/protocol"
	"github.com/cocodedk/parvaz/core/relay"
)

// TestRelay_Live_DoBatch_Roundtrips proves the batch path works
// against real Apps Script: one envelope, multiple inner fetches via
// UrlFetchApp.fetchAll, ordered responses.
func TestRelay_Live_DoBatch_Roundtrips(t *testing.T) {
	if os.Getenv("PARVAZ_LIVE") != "1" {
		t.Skip("set PARVAZ_LIVE=1 to run")
	}
	deploymentID, authKey := liveEnv(t)
	scriptURL := "https://script.google.com/macros/s/" + deploymentID + "/exec"
	r := liveRelay(t, authKey, scriptURL)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	start := time.Now()
	bresp, err := r.DoBatch(ctx, protocol.BatchRequest{Items: []protocol.Request{
		{Method: "GET", URL: "https://example.com/", FollowRedirects: true},
		{Method: "GET", URL: "https://example.org/", FollowRedirects: true},
		{Method: "GET", URL: "https://httpbin.org/get", FollowRedirects: true},
	}})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("DoBatch: %v", err)
	}
	if len(bresp.Items) != 3 {
		t.Fatalf("items = %d, want 3", len(bresp.Items))
	}
	t.Logf("3-item batch round-tripped in %s", elapsed)

	for i, item := range bresp.Items {
		if item.Err != nil {
			t.Errorf("item %d err: %v", i, item.Err)
			continue
		}
		if item.Response.Status != 200 {
			t.Errorf("item %d status = %d, want 200", i, item.Response.Status)
		}
		if len(item.Response.Body) == 0 {
			t.Errorf("item %d body empty", i)
		}
	}
	// example.com lives in slot 0; verify ordering survived round-trip.
	if !bytes.Contains(bytes.ToLower(bresp.Items[0].Response.Body), []byte("example domain")) {
		t.Errorf("item 0 (example.com) body lacks 'example domain'; first 200=%q",
			firstNBytes(bresp.Items[0].Response.Body, 200))
	}
}

// TestCoalescer_Live_AmortizesLatency is the headline Phase-2 metric:
// measure single-mode vs Coalescer-batched wall clock for the same
// concurrent workload, prove the batched path is materially faster
// against real Apps Script. Threshold is intentionally loose (batched
// must beat single-mode-sequential by ≥ 2×) to absorb network
// variance — the typical real-world ratio runs 5–8×.
func TestCoalescer_Live_AmortizesLatency(t *testing.T) {
	if os.Getenv("PARVAZ_LIVE") != "1" {
		t.Skip("set PARVAZ_LIVE=1 to run")
	}
	deploymentID, authKey := liveEnv(t)
	scriptURL := "https://script.google.com/macros/s/" + deploymentID + "/exec"
	r := liveRelay(t, authKey, scriptURL)

	const n = 6
	urls := []string{
		"https://example.com/",
		"https://example.org/",
		"https://httpbin.org/get",
		"https://example.com/",
		"https://example.org/",
		"https://httpbin.org/get",
	}

	// Baseline: N sequential single-mode invocations — what the
	// pre-Phase-2 runtime path produced.
	baselineStart := time.Now()
	for _, u := range urls {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, err := r.Do(ctx, protocol.Request{Method: "GET", URL: u, FollowRedirects: true})
		cancel()
		if err != nil {
			t.Fatalf("baseline Do %s: %v", u, err)
		}
	}
	baselineElapsed := time.Since(baselineStart)

	// Batched via Coalescer: N concurrent submissions, one envelope.
	c := relay.NewCoalescer(r, relay.CoalescerConfig{
		Window:   100 * time.Millisecond,
		MaxBatch: n,
	})
	defer c.Close()

	batchStart := time.Now()
	var wg sync.WaitGroup
	errs := make([]error, len(urls))
	for i, u := range urls {
		wg.Add(1)
		go func(i int, u string) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			_, errs[i] = c.Do(ctx, protocol.Request{Method: "GET", URL: u, FollowRedirects: true})
		}(i, u)
	}
	wg.Wait()
	batchElapsed := time.Since(batchStart)

	for i, err := range errs {
		if err != nil {
			t.Errorf("coalesced submit %d: %v", i, err)
		}
	}

	t.Logf("baseline (N=%d sequential single-mode): %s — per-request avg %s",
		n, baselineElapsed, baselineElapsed/time.Duration(n))
	t.Logf("batched  (N=%d concurrent via Coalescer): %s — per-request avg %s",
		n, batchElapsed, batchElapsed/time.Duration(n))
	ratio := float64(baselineElapsed) / float64(batchElapsed)
	t.Logf("speedup: %.2fx", ratio)

	if ratio < 2.0 {
		t.Errorf("Coalescer didn't materially amortize — ratio %.2fx, want ≥ 2.0x", ratio)
	}
}
