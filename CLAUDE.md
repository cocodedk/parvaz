# CLAUDE.md — Parvaz

## Project Overview

**Parvaz** (پرواز — Persian for "flight") is an Android VPN app built for
Iranian users with no technical background. A technical helper deploys a
**Cloudflare Worker** one-liner that forwards raw TCP traffic; a housewife
or factory worker installs Parvaz, scans a QR code or pastes a
`parvaz://` URL received via Telegram, and taps one button. System-wide,
Farsi-first, one-tap.

**Target audience**: Persian speakers with zero English, minimal tech
literacy. Every UI decision defers to this.

**Backend**: Cloudflare Workers with the `cloudflare:sockets` API. Raw TCP
passthrough means **no MITM**, no per-app cert trust issues, and full
HTTPS coverage — unlike URL-fetch-based relays (Apps Script, etc.) which
cannot tunnel opaque TLS.

**Circumvention stance**: TLS SNI is a popular Cloudflare-hosted domain;
HTTP Host header is the user's `*.workers.dev` deployment. Cloudflare's
edge routes by Host — politically expensive to block.

Monorepo. One APK. One worker.js. One release artifact.

---

## Shape

```
parvaz/
├── app/              Kotlin + Compose UI, Farsi-first, VpnService,
│                     tun2socks, sidecar-launcher, settings
├── core/             Go sidecar — fronter TLS dialer, WebSocket tunnel
│                     relay, local SOCKS5, main parvazd binary
├── worker/           Cloudflare Worker (worker.js, wrangler.toml)
├── reference/        MasterHttpRelayVPN (historical — product pivoted
│                     away from its Apps Script backend)
└── website/          Bilingual GitHub Pages (EN + FA)
```

**Core ↔ App boundary**: Go core cross-compiled per Android ABI to
`app/src/main/jniLibs/<abi>/libparvaz.so`. On launch, the app uses
`ProcessBuilder` to exec it, pipes a JSON config on stdin, waits for
`READY` on stdout, then talks to it as a **SOCKS5 server on
`127.0.0.1:1080`**. No JNI, no gomobile.

- **Min SDK**: 24 · **Target SDK**: 36 · **Kotlin**: 2.2.x · **AGP**: 9.1.x
- **UI**: Jetpack Compose Material3, **light only** (NOTAM aesthetic)
- **Go**: 1.24+ · **CGO_ENABLED**: 0

## UX — Farsi-first, zero-configuration

Every decision is driven by *"what does a Farsi-speaking factory worker
do next?"*. Default language is Persian (`fa`) regardless of device
locale. Everything is Farsi until a user explicitly toggles English.

**Onboarding — 3 screens total, never more.**
1. Splash + one-line trust: `پرواز` in giant Redaction, one `شروع` button.
2. Access import — one field, two buttons: `📋 چسباندن` (paste) and
   `📷 اسکن QR`. Auto-detect `parvaz://` in clipboard on launch.
3. VpnService permission prompt preceded by a Farsi explainer.

**Main screen — 1 button.**
- Disconnected: oxblood outlined `پرواز` rubber-stamp button.
- Connected: solid olive `در پرواز` + tiny `۰۰:۱۲:۴۷` uptime (Persian numerals).
- No settings menu, no stats, no logs, no account, no login.

**Access URL format**: `parvaz://<worker-host>/<access-key>#<optional-display-name>`.

The app registers the `parvaz://` URI scheme — Telegram/WhatsApp message
with a `parvaz://` link → tap → opens Parvaz already prefilled. Same for
QR codes.

---

## Layer rules

### Go core (`core/`)
- `core/fronter/` — TLS-with-custom-SNI dialer + HTTP client. Targets a Cloudflare edge IP; SNI is a popular CF-hosted domain; Host is the worker URL.
- `core/relay/` — WebSocket tunnel to the Worker. Implements `socks5.Dialer.Dial(ctx, host, port) (net.Conn, error)` — each SOCKS5 CONNECT opens one `wss://.../tunnel?k=<key>&host=<target>&port=<port>` and returns the WS as a `net.Conn` (binary-frame wrapper).
- `core/socks5/` — local SOCKS5 listener. Depends only on `relay/` via the `Dialer` interface.
- `core/cmd/parvazd/` — sidecar main. Reads JSON config on stdin.
- `core/protocol/`, `core/codec/` — reserved for a future control channel / optional HTTP interception. Not used by the raw TCP tunnel path.

### App (`app/`)
- `domain/` — pure Kotlin, no Android deps.
- `presentation/` — Compose + ViewModels. State is hoisted; screens take state + lambdas only.
- `vpn/` — `VpnService` subclass, `tun2socks` wrapper, sidecar launcher.
- `settings/` — `SharedPreferences` for relay URL and language; `EncryptedSharedPreferences` for access key.
- `ui/theme/` — NOTAM palette + Redaction/JetBrains Mono/Vazirmatn fonts.
- Single shared ViewModel at NavHost level.

---

## Visual identity — NOTAM parchment, Farsi-first

Light theme, never dynamic color. Palette: Paper `#F1E8D4`, Ink `#1A1410`,
Oxblood `#A8361C` (CTAs/errors), Burnt `#B5581A` (warnings), Olive
`#3A5634` (connected). Fonts: Vazirmatn (Persian display + body),
Redaction (Latin display, wordmark), JetBrains Mono (Latin code/numbers).

Connected button becomes a rubber-stamp `در پرواز`; disconnected is an
unstamped outline `پرواز`. Errors overlay diagonal oxblood stamps.

Invoke `frontend-design:frontend-design` before every new screen.

---

## Wire protocol

```
Per SOCKS5 CONNECT:
    dial <cloudflare_ip>:443         (TCP)
    TLS  SNI = <cf_front_domain>     (DPI-visible, a popular CF-hosted site)
    HTTP Host: <worker>.workers.dev  (Cloudflare edge routes by this)
    GET /tunnel?k=<key>&host=<t>&port=<p>
    Upgrade: websocket, Connection: Upgrade, ... (RFC 6455)

Worker accepts, opens connect({hostname:<t>, port:<p>}), pipes
WebSocket binary frames ↔ upstream TCP bidirectionally.
```

No per-request JSON envelope, no base64, no batching. TCP bytes are
opaque — the tunnel is transparent for HTTP, HTTPS, IMAP, XMPP, or
anything else.

---

## Reference implementation

`reference/` holds the upstream Python of MasterHttpRelayVPN — the project
Parvaz originally rewrote. Parvaz **pivoted away** from Apps Script (see
decision log in git history) because `UrlFetchApp.fetch()` cannot tunnel
opaque TLS and MITM breaks on Android. Kept for historical context only;
do not import.

---

## Memory (mem0 via user-scope MCP)

Every non-trivial session: `mcp__mem0__search_memory` scoped to
`project="parvaz"`, `user_id="bb"`. Persist durable facts with
`mcp__mem0__add_memory` at the same scope.

---

## Required skills

| Situation | Skill |
|---|---|
| Before any new feature | `superpowers:brainstorming` |
| Writing or fixing logic | `superpowers:test-driven-development` |
| First sign of a bug | `superpowers:systematic-debugging` |
| Before completing a feature | `superpowers:requesting-code-review` |
| Before declaring done | `superpowers:verification-before-completion` |
| Compose UI / new screens | `frontend-design:frontend-design` |
| After implementing — review | `simplify` |

---

## Engineering principles

- **200-line max per file** — extract helpers/packages when close. `reference/` + `website/` exempt.
- **TDD**: red → green → refactor.
- **Go stdlib first** — only unavoidable deps: `github.com/coder/websocket`, `github.com/andybalholm/brotli`, `github.com/klauspost/compress/zstd`.
- **No panics in Go library code** — errors only; panic just in `main`.
- **No secrets in logs** — never log access keys, worker URLs, or traffic content.
- **Explicit `context.Context`** on every Go network call.
- **State hoisted to ViewModel** in Compose — screens never touch `SharedPreferences` directly.
- **Kotlin immutable data classes** + `StateFlow`.
- **Farsi strings by default** — English lives in `values-en/` as the override locale, not the other way round.

---

## Build commands

```bash
go test -C core ./...                              # unit tests
go test -C core -race -cover ./...
CGO_ENABLED=0 GOOS=android GOARCH=arm64 \
    go build -C core -o ../app/src/main/jniLibs/arm64-v8a/libparvaz.so ./cmd/parvazd

./gradlew test
./gradlew assembleDebug
./gradlew buildSmoke

# Worker — deploy via wrangler (one-time setup in worker/)
cd worker && npx wrangler deploy
```

---

## Starting a new session

1. Read `CLAUDE.md`, `PLAN.md`, `ARCHITECTURE.md` in that order.
2. `go test -C core ./...` and `./gradlew test` to confirm baseline.
3. Invoke `superpowers:brainstorming` before touching any new milestone.
4. Next milestone: see `PLAN.md`.
