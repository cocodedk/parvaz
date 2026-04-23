# CLAUDE.md — Parvaz

## Project Overview

**Parvaz** (پرواز, Persian for "flight") is an Android VPN app that routes
phone traffic through a user-deployed **Google Apps Script** relay using
**domain fronting** (TLS SNI = `www.google.com`, HTTP `Host:` =
`script.google.com`). The user installs one APK, pastes their Apps Script
deployment URL + auth key, taps Connect, and the entire device routes
through their own Google-hosted relay.

Monorepo, one APK, one release artifact.

---

## Shape

```
parvaz/
├── app/              Kotlin + Compose UI, VpnService, tun2socks,
│                     sidecar-launcher, settings storage
├── core/             Go SOCKS5 server + domain-fronting transport
│                     (compiled to libparvaz.so, shipped in jniLibs)
├── reference/        Upstream Python (read-only study material)
└── website/          Bilingual GitHub Pages site (EN + FA)
```

**Core ↔ App boundary**: the Go core is compiled per Android ABI
(`arm64-v8a`, `armeabi-v7a`, `x86_64`, `x86`) and placed at
`app/src/main/jniLibs/<abi>/libparvaz.so`. On launch, the app uses
`ProcessBuilder` to exec the binary and talks to it as a **SOCKS5 server
on `127.0.0.1:1080`**. No JNI, no gomobile bindings.

- **Min SDK**: 24 (Android 7.0) | **Target SDK**: 36
- **Kotlin**: 2.2.x | **AGP**: 9.1.x | **Gradle**: 9.3.x (Kotlin DSL)
- **UI**: Jetpack Compose (Material3) | **Architecture**: MVVM + Clean
- **Go**: 1.22+ | **CGO_ENABLED**: 0

---

## Core layer rules (Go side, inside `core/`)

- `core/protocol/` — pure JSON envelope encode/decode. No network.
- `core/fronter/`  — TLS-with-custom-SNI dialer + HTTP/1.1 + h2 client. Shuttles bytes; never parses JSON.
- `core/codec/`    — gzip / br / zstd / identity decoders. No network.
- `core/relay/`    — glues envelope + client + codec. The only layer that speaks HTTP and JSON together.
- `core/socks5/`   — local SOCKS5 listener. Depends only on `relay/`.
- `core/cmd/parvazd/` — main sidecar binary. Reads config from stdin or flags; no business logic.

## App layer rules (Kotlin side, inside `app/`)

- **Domain layer** (`domain/`) — pure Kotlin, no Android deps, fully testable.
- **Presentation layer** (`presentation/`) — Compose screens + ViewModels + navigation.
- **VPN plumbing** (`vpn/`) — `VpnService` subclass, tun2socks wrapper, sidecar launcher.
- **Settings** (`settings/`) — `SharedPreferences` (relay URL) + `EncryptedSharedPreferences` (access key).
- **UI theme** (`ui/theme/`) — NOTAM palette + Redaction/Vazirmatn/JetBrains Mono fonts.
- **Single shared ViewModel** at NavHost level — all routes share one instance. Never call `viewModel()` inside a `composable { }` block.

---

## Visual identity — NOTAM / declassified flight briefing

The app carries the website's aesthetic so the product feels like one artifact.

- **Palette (light theme)**: Paper `#F1E8D4`, Ink `#1A1410`, Oxblood `#A8361C` (stamps, CTAs, errors), Burnt amber `#B5581A` (warnings), Olive `#3A5634` (connected/safe).
- **Fonts**: Redaction (display), JetBrains Mono (body), Vazirmatn (Persian), Redaction 35 (rubber-stamp chips). Bundle as `app/src/main/res/font/*`.
- **Signature moments**: Connect button as rubber stamp (`CLEAR FOR FLIGHT` → `GROUND FLIGHT`); connected state as telemetry (`T+00:12:47 · ALT 216.239.38.120`); errors as diagonal oxblood stamps over the offending field.
- Invoke `frontend-design:frontend-design` before every new screen. Do **not** use the naval-command default baked into `android-setup` — we override with NOTAM.

---

## Wire protocol (authoritative)

```
TCP:   dial <google_ip>:443           (default 216.239.38.120)
TLS:   SNI = www.google.com           (DPI sees this)
HTTP:  Host: script.google.com        (Google routes by this)
       POST /macros/s/<DEPLOYMENT_ID>/exec HTTP/1.1
       Content-Type: application/json
```

Request body (single): `{ "k", "m", "u", "h", "b", "ct", "r" }` (auth key, method, url, headers, base64 body, content-type, follow-redirects).
Batch: `{ "k", "q": [ {...}, {...} ] }`. Response: `{ "s", "h", "b" }` or `{ "e": "unauthorized" }`.

Skipped headers (server strips): `host`, `connection`, `content-length`, `transfer-encoding`, `proxy-*`, `priority`, `te`.

---

## Reference implementation

`reference/` holds upstream Python — **read-only**. Study for protocol details; do not import. Key files: `apps_script/Code.gs` (server contract), `src/domain_fronter.py` (SNI/Host split + pooling), `src/h2_transport.py` (h2 multiplexing), `src/codec.py` (content-encoding).

---

## Memory (mem0 via user-scope MCP)

Start every non-trivial session with `mcp__mem0__search_memory` scoped to `project="parvaz"`, `user_id="bb"`. Persist durable facts with `mcp__mem0__add_memory` at the same scope. Skip anything captured in code, git log, or this file.

---

## Required Skills — ALWAYS invoke

| Situation | Skill |
|---|---|
| Before any new feature | `superpowers:brainstorming` |
| Planning multi-step changes | `superpowers:writing-plans` |
| Writing or fixing logic (Go or Kotlin domain) | `superpowers:test-driven-development` |
| First sign of a bug or build failure | `superpowers:systematic-debugging` |
| Before completing a feature branch | `superpowers:requesting-code-review` |
| Before claiming any task done | `superpowers:verification-before-completion` |
| Compose UI / new screens | `frontend-design:frontend-design` |
| After implementing — quality review | `simplify` |

---

## Engineering principles

- **200-line max per file** — extract helpers/composables/packages when approaching the limit. Applies to `.kt`, `.go`, `.md`, `.html`, `.css`, `.yml`, `.sh`. Exempt: `reference/`, generated files, `website/`.
- **TDD**: red → green → refactor. One failing test, minimal implementation.
- **Go stdlib first** — `crypto/tls`, `net/http`, `encoding/json`, `compress/gzip` cover most of it. Brotli + zstd need third-party.
- **No panics in Go library code** — return errors; panic only in `main`.
- **No secrets in logs** — never log `auth_key`, deployment URL, or phone traffic content.
- **Explicit `context.Context`** on every Go network call.
- **State hoisted to ViewModel** in Compose — screens take state + lambdas, never touch `SharedPreferences` directly.
- **Immutable domain models** in Kotlin — `data class` + `copy()`.
- **ViewModels expose `StateFlow`** — `MutableStateFlow` never public.
- **No Conscrypt hacks** — all TLS / SNI / fingerprint work lives in Go.

---

## Build commands

```bash
# Go core (from core/ or repo root with -C flag)
go test -C core ./...                     # unit tests
go test -C core -race -cover ./...        # race + coverage
go vet  -C core ./...                     # static checks

# Cross-compile core for Android ABIs
CGO_ENABLED=0 GOOS=android GOARCH=arm64 \
    go build -C core -o ../app/src/main/jniLibs/arm64-v8a/libparvaz.so ./cmd/parvazd

# Android app
./gradlew test                            # unit tests
./gradlew assembleDebug                   # debug APK
./gradlew assembleRelease                 # release APK (needs signing env)
./gradlew buildSmoke                      # debug + test + lint (CI)

# Live-network Go tests (needs deployed Code.gs)
PARVAZ_E2E=1 go test -C core ./...
```

---

## Starting a new session

1. Read `CLAUDE.md`, `PLAN.md`, `ARCHITECTURE.md` in that order.
2. Run `go test -C core ./...` and `./gradlew test` to confirm baseline passes.
3. Invoke `superpowers:brainstorming` before touching any new milestone.
4. Pick the next unchecked milestone in `PLAN.md` and write the failing test first.
