# Throughput optimization — Go side wins, Code.gs is near-optimal

**Status:** Phases 1, 2, 4 landed on `perf-throughput` · 2026-04-26
**Baseline:** ~5 KB/s through the Apps Script tunnel.
**Target:** 5–20× gain by fixing the Go side; Code.gs only needs minor cleanups.

| Phase | What | State |
|---|---|---|
| 1 | Fronter pool tunables | ✅ landed |
| 2 | DoBatch + Coalescer + wired into MITM/DoH paths | ✅ landed |
| 3 | h2 ALPN spike | ⏸ deferred — Phase 2 hit target |
| 4 | Code.gs micro-cleanups + matching client-side accept-encoding strip | ✅ landed |

## Measured (2026-04-26, live test server)

`TestCoalescer_Live_AmortizesLatency` with N=6 against real Apps Script:

| Path | Wall clock | Per-request |
|---|---|---|
| Baseline (sequential single-mode) | 9.01 s | 1.50 s |
| Coalescer-batched (1 envelope) | 1.81 s | 302 ms |

**Speedup: 4.97×** — lower end of the 5–20× range, which is expected
for N=6 (more concurrency amortizes more, up to `MaxBatch=8`). The
1.50 s/req baseline matches the analysis's "300–1500 ms Apps Script
fixed cost" — now spread across the batch instead of paid per request.

---

## TL;DR

The bottleneck is **not Code.gs** — it's the Go relay stack. Code.gs
already exposes a batch endpoint (`UrlFetchApp.fetchAll` in parallel
inside one Apps Script invocation), but the runtime never calls it. On
top of that, the fronted HTTP/1.1 client uses Go's default
`MaxIdleConnsPerHost = 2`, so even single-mode requests serialize
through 2 sockets. Three phases land independently; each can be merged
on its own.

---

## What was inspected

- `core/protocol/encode.go` — `EncodeBatch` exists, fully tested.
- `core/protocol/decode.go` — `DecodeBatchResponse` exists, fully tested.
- `core/relay/relay.go` (line 58) — `Do()` only calls `EncodeSingle`.
- `core/cmd/parvazd/relay_rt.go` (line 29) — RoundTripper used by the
  MITM path issues one `relay.Do` per `http.Request`. No coalescing.
- `core/fronter/client.go` — `http.Transport{}` with no
  `MaxIdleConnsPerHost`, no `MaxConnsPerHost`, no h2.
- `apps_script/Code.gs` — `_doBatch` reachable via top-level
  `q: [...]` field; uses `UrlFetchApp.fetchAll`; correctly preserves
  per-item index + per-item errors.

Conclusion: **server-side batching is wired and working; the Go side
just never sends a batch envelope.**

---

## Bottleneck breakdown

### 1. Apps Script per-invocation fixed cost (~300–1500 ms)

V8 cold-start + Apps Script auth/quota overhead + Google edge RTT.
Structural — can't be removed, only amortized. Today every browser
HTTP request pays this in full.

**Mitigation:** batch N small requests into one envelope. With N≈8 and
fixed cost ≈800 ms, per-request cost drops from ~800 ms to ~100 ms.
This is the biggest lever.

### 2. HTTP/1.1 head-of-line blocking on 2 sockets

`core/fronter/client.go:23` builds an `http.Transport` with no
connection-pool tunables. `net/http` defaults to
`MaxIdleConnsPerHost = 2` — every fronted POST to
`216.239.38.120:443` queues behind at most 2 in-flight TLS sockets.

**Mitigation:** raise `MaxIdleConnsPerHost` (e.g. 16) and set
`MaxConnsPerHost` to a sane upper bound. This is a one-file change
with one assertion test.

### 3. base64 inflation (~33 %)

`Utilities.base64Encode(resp.getContent())` in Code.gs:53 is forced —
`ContentService` cannot return binary. No workaround at the
Apps Script layer.

**Status:** accepted tax. Document it, move on.

### 4. Idle-conn TLS reconnect at 90 s

`IdleConnTimeout: 90 * time.Second` means each idle window costs a
fresh TLS handshake to a fronted IP with measurable RTT.

**Status:** secondary; revisit only if Phase 1 + 2 don't hit target.

---

## Plan

### Phase 1 — Fronter pool tunables (low risk, high reward)

**Slice:** raise `MaxIdleConnsPerHost`, set `MaxConnsPerHost`, expose
both as `NewHTTPClient` options with sane defaults.

**TDD:**
1. Red: `TestNewHTTPClient_TransportPoolDefaults` asserts the
   transport's `MaxIdleConnsPerHost` ≥ 8 (today: 0 → defaults to 2).
2. Implement: set the fields on the constructed `http.Transport`.
3. Existing four `client_test.go` tests must stay green.

**Acceptance:** unit test passes; SOCKS5 → relay path still serves a
real fronted POST in `TestRelay_*` integration tests.

**Risk:** none — pool sizing is a pure resource knob.

### Phase 2 — Wire batch path through relay (high risk, biggest win)

The runtime path is per-request synchronous (`http.RoundTripper.RoundTrip`).
Batching requires a **coalescer**: incoming requests wait up to a small
time window (e.g. 5–15 ms) or a max-batch-size, then ship as one
`EncodeBatch` envelope. Responses fan out by index.

**TDD slices, in order:**

1. **`relay.Relay.DoBatch(ctx, BatchRequest)`** — pure protocol path.
   - Red: `TestRelay_DoBatch_RoundTrips` — feed a 3-item batch, assert
     3 responses in order, plus the stub recorded **one** envelope hit
     (proves it's a real batch, not 3 singles).
   - Blocker: `testutil.AppsScriptStub` is single-mode only today.
     Extend to recognize top-level `q:[...]` and reply with `{q:[...]}`
     envelopes. Mirror Code.gs's `_doBatch` semantics including
     per-item errors.
   - Implement `DoBatch` in `relay.go`: encode → POST → decode batch
     response → return `*BatchResponse`.
2. **`relay.Coalescer`** — per-relay queue + flush trigger.
   - `Submit(ctx, Request) (*Response, error)` blocks until the
     coalesced batch returns this caller's slot.
   - Tests:
     - 1 request flushes immediately if window ≤ 0.
     - N requests within window flush together (one stub hit).
     - Window expiry flushes pending.
     - Per-item server error returns to *that* caller, others succeed.
     - Cancellation of one caller's ctx doesn't poison others.
   - Configurable `Window time.Duration` and `MaxBatch int`.
3. **Wire `Coalescer` into the relay roundtripper.**
   - `relayRoundTripper.RoundTrip` calls `coalescer.Submit` instead of
     `relay.Do`.
   - Existing relay-RT tests stay green; add one that hits the same
     RT 5× concurrently and asserts the stub saw 1 batch hit, not 5.

**Acceptance:** the batch path is the default; single-mode stays
available for the diagnostic relay tests. Browser end-to-end smoke
shows fewer Apps Script invocations per page load.

**Risk:** moderate — ordering, cancellation, and partial-failure
semantics need care. Hence the slicing.

**Defaults to start with:**
- `Window = 10 * time.Millisecond`
- `MaxBatch = 8`
Both tunable via JSON config so we can A/B without recompiling.

### Phase 3 — h2 ALPN spike (deferred — Phase 2 hit target)

Enable ALPN `h2` in the fronter's TLS config so multiplexing dissolves
the per-conn HOL bottleneck. Browser-facing TLS layer stays http/1.1
intentionally (`docs/tls-flow.md` §3 — h2 enables SNI coalescing
across hostnames, which would break per-host leaf certs).

### Phase 4 — Code.gs micro-cleanups

Pure noise reduction, no perf impact:
- Drop `validateHttpsCertificates: true` (default).
- Drop `escaping: false` (default).
- Drop the `getHeaders()` fallback in `_respHeaders` — modern
  UrlFetchApp always exposes `getAllHeaders`.
- Add `accept-encoding` to `SKIP_HEADERS` — saves a few bytes
  upstream; the Google frontend handles encoding negotiation.

**Deferred:** `CacheService.getScriptCache()` for idempotent GETs of
static assets. Real win, but changes semantics (staleness, cookie
leaks) — wants a design discussion first.

---

## Out of scope

DPI / fronting strategy. Switching off Apps Script. MITM-leg ALPN
(would break per-host leaf reuse — see `docs/tls-flow.md` §3).

## Rollback

Phase 1: revert the field set. Phase 2: set `MaxBatch = 1` —
degenerates to single-mode + one window tick. Phase 3: drop the flag.
