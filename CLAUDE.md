# CLAUDE.md — Parvaz

## Project Overview

**Parvaz** (پرواز — Persian for "flight") is a Farsi-first Android app that
tunnels **browser traffic** through a user-deployed Google Apps Script
relay, using TLS SNI concealment and a local MITM to terminate TLS for
re-encapsulation. A technical helper deploys `Code.gs` on their own
Google account once; a non-technical user installs Parvaz, pastes or
scans a `parvaz://` URL, installs a user-CA via Android Settings, and
taps one button.

**Architecture matches the proven MasterHttpRelayVPN-RUST port**:
[github.com/therealaleph/MasterHttpRelayVPN-RUST](https://github.com/therealaleph/MasterHttpRelayVPN-RUST).
Parvaz's differentiators are the NOTAM visual identity, Farsi-by-default
UI, and tighter onboarding for non-technical users.

**Honest scope (per MITM reality on Android 7+):**
- ✅ **Chrome (and other Chromium browsers — Brave, Edge, Vivaldi)** —
  trust the user-installed CA out of the box, no flags. Chrome's CT
  enforcement has an explicit carve-out for chains terminating in a
  user CA, so MITM is transparent. **Recommended browser.**
- ✅ **Google-owned hosts in any browser** — google.com, youtube.com,
  etc. take the SNI-rewrite path without MITM; no quota, no cert
  needed.
- ⚠️ **Firefox Nightly only** — needs `security.enterprise_roots.enabled`
  flipped in `about:config`. Stable/Beta hide `about:config` entirely;
  Nightly resets the pref to `false` on every restart (known Mozilla
  bug [fenix#18990](https://github.com/mozilla-mobile/fenix/issues/18990),
  open since 2021). Effectively unusable — do not recommend in user
  docs without a heavy caveat.
- ❌ **DuckDuckGo browser** — confirmed broken; rejects the user CA
  despite being Chromium-based.
- ❌ **Instagram / Telegram / WhatsApp / banking / streaming native apps**
  — these reject user-CAs and break. This is a general Android-MITM
  constraint, not a Parvaz bug. Users who need those apps pair Parvaz
  with xray+VLESS pointing at their own VPS.

Monorepo. One APK. One `Code.gs`.

---

## Shape

```
parvaz/
├── app/              Kotlin + Compose UI, Farsi-first, VpnService +
│                     tun2socks + sidecar-launcher + CA install flow
├── core/             Go sidecar — fronter TLS dialer, MITM interceptor,
│                     Apps Script envelope relay, local SOCKS5, parvazd
├── reference/        Upstream MasterHttpRelayVPN Python (read-only study)
└── website/          Bilingual GitHub Pages (EN + FA)
```

**Core ↔ App boundary**: Go core cross-compiled per ABI to
`app/src/main/jniLibs/<abi>/libparvaz.so`. App uses `ProcessBuilder` to
exec, pipes JSON config on stdin, reads `READY` on stdout, then talks to
it as a **SOCKS5 server on `127.0.0.1:1080`**. No JNI, no gomobile.

- **Min SDK**: 24 · **Target SDK**: 36 · **Kotlin**: 2.2.x · **AGP**: 9.1.x
- **UI**: Compose Material3, **light only** (NOTAM parchment)
- **Go**: 1.24+ · **CGO_ENABLED**: 0

---

## UX — Farsi-first, zero-configuration

Default language is Persian (`fa`) always, not detected. English lives
as an override locale.

**Onboarding — 4 screens** (CA install is the unavoidable extra step):
1. Splash + `شروع`.
2. Access import — paste `parvaz://` or scan QR.
3. **MITM CA install** — Farsi walkthrough that triggers Android's CA
   install intent; verifies afterwards via `AndroidCAStore`. Requires
   screen-lock set; we prompt if missing.
4. VpnService permission prompt, preceded by a Farsi explainer.

**Main screen** — 1 button (rubber stamp, NOTAM aesthetic):
- Disconnected: oxblood outline `پرواز`.
- Connected: olive solid `در پرواز` + `T+۰۰:۱۲:۴۷` (Persian numerals).

**Access URL**: `parvaz://<deployment-id>/<access-key>#<display-name>`.
The app registers the URI scheme — Telegram link tap → opens Parvaz
prefilled. QR scanner takes the same format.

---

## Layer rules

### Go core (`core/`)
- `core/protocol/` — Apps Script JSON envelope encode/decode (single + batch). Pure, no network.
- `core/fronter/` — TLS-with-custom-SNI dialer + HTTP/1.1 client. Target = Google edge IP; SNI = `www.google.com`; Host (set by caller) = `script.google.com`.
- `core/codec/` — gzip / br / zstd decoders for Apps Script responses.
- `core/relay/` — glues envelope + fronted client + codec. One `Do(ctx, req)` call per tunneled HTTP request.
- `core/mitm/` (new, M-next) — CA generation + persistence, dynamic leaf cert per target, TLS server that terminates TLS locally so request bytes become inspectable HTTP.
- `core/dispatcher/` (new, M-next) — decides per target whether to use MITM+relay (the default) or SNI-rewrite direct tunnel (for `*.google.com`, `*.youtube.com`, `fonts.googleapis.com`, etc.).
- `core/socks5/` — local SOCKS5 listener. CONNECT-only, no auth. Calls into `dispatcher`.
- `core/cmd/parvazd/` — sidecar main. Reads JSON config on stdin.

### App (`app/`)
- `domain/` — pure Kotlin, no Android deps.
- `presentation/` — Compose + ViewModels. State hoisted; screens take state + lambdas only.
- `vpn/` — `VpnService` subclass, `tun2socks` wrapper, sidecar launcher.
- `settings/` — parvaz:// parser (done — M11), `SharedPreferences` for relay URL and language, `EncryptedSharedPreferences` for access key.
- `mitm/` — CA export to `Downloads/mhrv-ca.crt`-equivalent, intent to `ACTION_MANAGE_CA_CERTIFICATES`, post-install fingerprint verification against `AndroidCAStore`.
- `ui/theme/` — NOTAM palette + Redaction / JetBrains Mono / Vazirmatn.
- Single shared ViewModel at NavHost level.

---

## Visual identity — NOTAM parchment

Paper `#F1E8D4`, Ink `#1A1410`, Oxblood `#A8361C` (CTAs/errors), Burnt
`#B5581A` (warnings), Olive `#3A5634` (connected). Fonts: Vazirmatn
(Persian body/display), Redaction (Latin display), JetBrains Mono
(code). Light theme, never dynamic. Connected button = rubber-stamp
`در پرواز`; disconnected = outline `پرواز`. Errors overlay diagonal
oxblood stamps. Invoke `frontend-design:frontend-design` before every
new screen.

---

## Wire protocol — Apps Script

```
Per tunneled HTTP request:
    TCP connect  <google_ip>:443        (default 216.239.38.120)
    TLS  SNI = www.google.com           (DPI-visible)
    POST  https://script.google.com/macros/s/<id>/exec  HTTP/1.1
    Content-Type: application/json
    body = { k, m, u, h, b?, ct?, r }  (see protocol/envelope.go)
```

Response: `{s, h, b}` or `{e: "unauthorized"}`. Batch mode available.

For Google-owned targets: skip the envelope. Direct TCP to google_ip
with SNI = `www.google.com`, HTTP `Host: <target>` — Google's edge
routes by Host. No Apps Script quota consumed, no MITM needed.

---

## Reference

`reference/` holds upstream Python. **Read-only** — study for protocol
details. Also see [MasterHttpRelayVPN-RUST](https://github.com/therealaleph/MasterHttpRelayVPN-RUST)
for a mature Rust port we're architecturally aligned with.

---

## Memory (mem0 via user-scope MCP)

Every non-trivial session: `mcp__mem0__search_memory` scoped to
`project="parvaz"`, `user_id="bb"`. Persist durable facts via
`mcp__mem0__add_memory`.

---

## Required skills

| Situation | Skill |
|---|---|
| Before any new feature | `superpowers:brainstorming` |
| Writing or fixing logic | `superpowers:test-driven-development` |
| First sign of a bug | `superpowers:systematic-debugging` |
| Before declaring done | `superpowers:verification-before-completion` |
| Compose UI / new screens | `frontend-design:frontend-design` |
| After implementing — review | `simplify` |

---

## Engineering principles

- **200-line max per file** — extract when close. `reference/` + `website/` exempt.
- **TDD**: red → green → refactor.
- **Go stdlib first** — deps: `github.com/andybalholm/brotli`, `github.com/klauspost/compress/zstd`.
- **No panics in Go library code** — errors only; panic in `main`.
- **No secrets in logs** — never log access keys, deployment URLs, or traffic content.
- **Explicit `context.Context`** on every Go network call.
- **State hoisted to ViewModel** in Compose.
- **Farsi strings by default** — English lives in `values-en/`.

---

## Build commands

```bash
go test -C core ./...
go test -C core -race -cover ./...
CGO_ENABLED=0 GOOS=android GOARCH=arm64 \
    go build -C core -o ../app/src/main/jniLibs/arm64-v8a/libparvaz.so ./cmd/parvazd

./gradlew test
./gradlew assembleDebug
./gradlew buildSmoke

# Instrumentation tests (androidTest/) — Android emulator must be running.
# Use for anything that needs a real Android runtime: CoreLauncher, CA
# install + AndroidCAStore verify, tun2socks, VpnService. Host JVM tests
# (./gradlew test) remain the default for domain + pure-Kotlin logic.
./gradlew connectedAndroidTest
```

---

## Starting a new session

1. Read `CLAUDE.md`, `PLAN.md`, `ARCHITECTURE.md`.
2. `go test -C core ./...` and `./gradlew test`.
3. Invoke `superpowers:brainstorming` before any new milestone.
4. Next milestone: see `PLAN.md`.
