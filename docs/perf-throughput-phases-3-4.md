# Throughput optimization, phases 3 and 4

Moved out of [perf-throughput.md](perf-throughput.md) to keep that file within the 200-line limit; the text is unchanged.

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
