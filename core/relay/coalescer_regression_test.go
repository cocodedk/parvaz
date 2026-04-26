package relay_test

// Regression coverage for the simplify-pass review fixes:
//   - Coalescer.Close() must cancel the in-flight upstream POST via
//     the batch ctx. Without this, a hung Apps Script call wedges
//     a goroutine until ResponseHeaderTimeout (30 s).
//   - dispatch must guard against length mismatch between the sent
//     batch and the server's q-array. Without the guard, a malformed
//     reply would panic on bresp.Items[i].

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cocodedk/parvaz/core/fronter"
	"github.com/cocodedk/parvaz/core/protocol"
	"github.com/cocodedk/parvaz/core/relay"
)

// coalescerWithHandler builds a Coalescer that talks to a custom
// httptest.Server with the given handler, so tests can simulate
// pathological server behavior the AppsScriptStub doesn't model.
func coalescerWithHandler(t *testing.T, h http.HandlerFunc, cfg relay.CoalescerConfig) (*relay.Coalescer, *httptest.Server) {
	t.Helper()
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	d := &fronter.Dialer{FrontDomain: "www.google.com", InsecureSkipVerify: true}
	client := fronter.NewHTTPClient(d, u.Host)
	r, err := relay.New(relay.Config{
		HTTPClient: client,
		ScriptURLs: []string{srv.URL + "/macros/s/X/exec"},
		AuthKey:    "k",
	})
	if err != nil {
		t.Fatal(err)
	}
	return relay.NewCoalescer(r, cfg), srv
}

// Close() must propagate cancellation through the batch ctx so a
// pending DoBatch HTTP call returns instead of riding ResponseHeader-
// Timeout. Without the simplify-pass batchCtx wiring, this test would
// hang for ~30 s before the Submit goroutine returns.
func TestCoalescer_Close_CancelsInFlightBatch(t *testing.T) {
	// defer (not t.Cleanup) so this fires BEFORE the httptest.Server's
	// own t.Cleanup(srv.Close), which would otherwise block forever
	// waiting for the still-blocked handler to return.
	released := make(chan struct{})
	defer close(released)

	hits := make(chan struct{}, 1)
	c, _ := coalescerWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case hits <- struct{}{}:
		default:
		}
		select {
		case <-released:
		case <-r.Context().Done():
		}
	}, relay.CoalescerConfig{Window: 5 * time.Millisecond, MaxBatch: 1})

	submitDone := make(chan error, 1)
	go func() {
		_, err := c.Do(context.Background(), protocol.Request{
			Method: "GET", URL: "https://x", FollowRedirects: true,
		})
		submitDone <- err
	}()

	select {
	case <-hits:
	case <-time.After(2 * time.Second):
		t.Fatal("server never received the batched request")
	}

	closeStart := time.Now()
	c.Close()
	closeElapsed := time.Since(closeStart)
	if closeElapsed > 2*time.Second {
		t.Errorf("Close took %s, want < 2s — batch ctx not propagating", closeElapsed)
	}

	select {
	case err := <-submitDone:
		if err == nil {
			t.Error("Submit returned nil after Close, want a context error")
		}
	case <-time.After(2 * time.Second):
		t.Error("Submit never returned after Close — batch ctx didn't cancel the HTTP call")
	}
}

// dispatch must error all callers with a length-mismatch message when
// the server replies with fewer (or more) q items than were sent.
// The guard prevents an out-of-range panic that would crash the run
// loop's flusher goroutine.
func TestCoalescer_BatchLengthMismatch_FailsAllCallers(t *testing.T) {
	c, _ := coalescerWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Reply with exactly 1 item regardless of how many were sent.
		_, _ = w.Write([]byte(`{"q":[{"s":200,"h":{},"b":""}]}`))
	}, relay.CoalescerConfig{Window: 30 * time.Millisecond, MaxBatch: 8})
	defer c.Close()

	const n = 3
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = c.Do(context.Background(), protocol.Request{
				Method: "GET", URL: "https://x", FollowRedirects: true,
			})
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err == nil {
			t.Errorf("submit %d: want error, got nil", i)
			continue
		}
		if !strings.Contains(err.Error(), "batch length mismatch") {
			t.Errorf("submit %d: err = %v, want one mentioning 'batch length mismatch'", i, err)
		}
	}
}
