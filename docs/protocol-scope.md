# Protocol scope — what works on which path

Parvaz routes each browser CONNECT through one of three dispatcher
paths (`core/dispatcher/dispatcher.go`). The path determines whether
arbitrary application protocols work end-to-end, because two of the
three paths are pure byte pipes while the third terminates HTTP and
re-encapsulates each request as an Apps Script RPC.

| Path | Hosts | Mechanism | Bidirectional bytes? |
|---|---|---|---|
| 1 — direct TCP | `*.google.com`, `*.googleusercontent.com`, `*.gstatic.com`, `*.googleapis.com` | Raw `io.Copy` after SOCKS5 CONNECT | ✅ yes |
| 2 — SNI-rewrite | `*.youtube.com`, `*.ytimg.com`, `*.ggpht.com` | Browser TLS terminated locally, fresh upstream TLS with safe SNI, raw `io.Copy` between | ✅ yes |
| 3 — MITM + Apps Script | catch-all (everything else) | Browser TLS terminated, each `http.ReadRequest` becomes one `Relay.Do(...)` POST to Apps Script's `UrlFetchApp` | ❌ no — request/response only |

## What works on Path 3 (the catch-all)

- ✅ Plain HTTPS GET/POST/PUT/DELETE/PATCH — anything that fits the
  classic request/response shape
- ✅ HTTP redirects (303, 302, 307, 308) when `FollowRedirects=true`
- ✅ Compressed bodies (gzip, brotli, zstd) — decoded by `core/codec`
- ✅ Set-Cookie + repeated headers — preserved by `protocol.respHeaders`
- ✅ Large bodies up to Apps Script's payload limit (~50 MB request,
  unbounded response — but base64 inflates ~33 %)

## What does NOT work on Path 3

These all need persistent bidirectional bytes between browser and
upstream, which `UrlFetchApp` fundamentally cannot provide — it's a
one-shot HTTP RPC with no socket-upgrade primitive.

- ❌ **WebSockets** (`Upgrade: websocket`) — Apps Script may forward
  the Upgrade header, but it can't hold the connection open after the
  101 response, and even if it did, the relay loop reads the next
  HTTP request from the browser, not WS frames
- ❌ **HTTP/2 server push, gRPC streaming** — same reason
- ❌ **Long-poll / Server-Sent Events (SSE)** — request times out at
  Apps Script's per-invocation cap (~6 min) and the browser sees a
  truncated stream
- ❌ **CONNECT to non-443 ports proxied through HTTPS** — only `:443`
  is on the path; everything else is dispatcher-default-rejected

## What works on Paths 1 + 2 (Google-owned hosts)

Everything. The dispatcher hands the SOCKS5-CONNECT'ed conn to a raw
byte pipe, so any protocol the browser speaks works end-to-end:

- ✅ WebSockets to `*.google.com` (Drive realtime, Meet signaling, …)
- ✅ YouTube DASH/HLS streaming via `*.googlevideo.com`
- ✅ Google Cloud Console gRPC-Web (over HTTPS to `*.googleapis.com`)

## Implication for app design

If you're building something on top of Parvaz that needs WebSockets
or streaming, route the WS endpoint through a Google-owned proxy
(e.g. host the WS server behind Cloud Run on `*.run.app` —
which is *.googleusercontent.com-adjacent and would need an
allow-list addition; check first).

For pure browsing (the primary use case) the limitation is invisible
— most sites that *use* WS for chat widgets degrade to HTTP polling
when the WS handshake fails, which Path 3 handles fine.

## Where this is enforced in code

- `core/dispatcher/dispatcher.go` — picks the path
- `core/mitm/interceptor.go` — Path 3's TLS-terminate + http.ReadRequest loop
- `core/mitm/snitunnel.go` — Path 2's raw byte pipe
- `core/protocol/types.go` — `skipHeaders` strips hop-by-hop headers,
  but `Upgrade` is preserved (so the upstream sees the attempt and
  fails cleanly with a non-101 response)
