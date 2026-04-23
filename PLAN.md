# Implementation Plan — Parvaz

An Android VPN app (Kotlin + Compose, Farsi-first) that tunnels all phone
traffic through a user-deployed Cloudflare Worker via a raw TCP passthrough
WebSocket. One APK. One worker.js. Three phases: Go core, Android app,
integration.

---

## Milestone 0 — Skeleton

- [x] `reference/` cloned (historical MasterHttpRelayVPN Python, read-only)
- [x] `CLAUDE.md`, `PLAN.md`, `ARCHITECTURE.md` — project docs
- [x] `core/go.mod` — module `github.com/cocodedk/parvaz/core`
- [x] `.gitignore`, `LICENSE` (MIT), `version.txt`, GitHub scaffolding
- [x] Android Studio project scaffolded into `app/` with NOTAM theme
- [x] `git init` + `cocodedk/parvaz` public repo live

---

# Phase A — Go Core (Cloudflare Worker edition)

## Milestone 1 — ~~protocol envelope~~ [RESERVED — not on critical path]

`core/protocol/` is implemented + tested (7 passes, 81% cov) but UNUSED
in the Cloudflare tunnel path — TCP is opaque and needs no JSON envelope.
Package kept for a possible future control channel (stats push, version
negotiation). Safe to leave as-is or delete.

## Milestone 2 — `core/fronter` dialer

- [x] `Dialer{FrontDomain, InsecureSkipVerify, BaseDialer, TLSConfigHook}`
- [x] 3 tests: SNI split, unreachable error, context cancellation
- FrontDomain now defaults to a popular Cloudflare-hosted domain (TBD);
      target is a Cloudflare edge IP (e.g. `104.16.0.0`).

## Milestone 3 — `core/fronter` HTTP client

- [x] `NewHTTPClient(d, target) → *http.Client`
- [x] 4 tests: SNI/Host split, POST echo, 503, context deadline
- Used by Milestone 5 for the WebSocket upgrade handshake.

## Milestone 4 — `core/codec` [RESERVED — not on critical path]

`core/codec/` is implemented + tested (6 passes, 94% cov) but UNUSED in
the TCP tunnel path. Kept for a future HTTP-intercepting mode. Safe to
leave as-is or delete.

## Milestone 5 — `core/relay` · WebSocket TCP tunnel

**Rewritten** from the Apps Script JSON envelope. Now a WebSocket dialer.

Target: `core/relay/relay.go` + `core/relay/relay_test.go` +
`core/internal/testutil/worker_stub.go`.

API:
```
type Config struct {
    HTTPClient *http.Client   // uses fronter internally
    WorkerURL  string         // wss://x.workers.dev/tunnel
    AuthKey    string
}

func New(cfg Config) (*Relay, error)
func (r *Relay) Dial(ctx, host string, port uint16) (net.Conn, error)
```

`Dial` opens `wss://<worker>/tunnel?k=<key>&host=<host>&port=<port>`,
returns the WebSocket wrapped as a `net.Conn` (binary-frame stream).
Implements `socks5.Dialer`.

Dep: `github.com/coder/websocket`.

Failing-test order:
1. `TestRelay_Dial_PassesAuthAndTarget` — stub records query params; one Dial.
2. `TestRelay_Dial_Unauthorized_ReturnsError` — stub returns 401; Dial errors.
3. `TestRelay_Dial_ProxiesTCPBytesBothWays` — stub echoes; conn round-trips.
4. `TestRelay_Dial_PropagatesContextCancel` — stub hangs; ctx deadline returns.
5. `TestRelay_Dial_ServerCloseCausesConnEOF` — stub closes; Read → EOF.

## Milestone 6 — `core/socks5` listener

- [x] Minimal SOCKS5, no auth, CONNECT-only.
- [x] 5 tests / 69.5% cov.
- No code changes needed — the `Dialer` interface M5 implements is exactly what M6 consumes.

## Milestone 7 — WebSocket stream multiplexing (optimization)

Later. Right now each CONNECT opens its own WS. h2 multiplexing over a
single shared TLS connection reduces TCP+TLS overhead on the device.

## Milestone 8 — ~~request batching~~ [REMOVED]

No longer applicable — the TCP passthrough tunnel has no request boundary to batch.

## Milestone 9 — `core/cmd/parvazd` wiring

- [x] Binary builds, prints `READY`, listens on :1080.
- [ ] Replace `stubDialer` with a configured `relay.Relay` instance.
- [ ] Parse `parvaz://` URLs in stdin config (host + key).
- [ ] Smoke test: pipe config, call SOCKS5 CONNECT, verify tunnel opens.

---

# Phase B — Android App (Farsi-first)

Depends on a compiled `libparvaz.so` in `app/src/main/jniLibs/<abi>/`.

## Milestone 10 — Compose NOTAM theme

- [x] `ui/theme/Color.kt` / `Theme.kt` / `Type.kt` — NOTAM palette, light-only.
- [x] `app/src/main/res/values/strings.xml` — placeholder.
- [ ] Bundle **Vazirmatn** (required — Persian body font) in `res/font/`.
- [ ] Bundle **Redaction** + **JetBrains Mono** in `res/font/`.
- [ ] Switch Type.kt from placeholder FontFamily.Serif/Monospace to the bundled fonts.
- [ ] Persian-aware typography scale (Vazirmatn letter-spacing = 0, Latin 2sp+).

## Milestone 11 — Settings + parvaz:// URL parser

- `settings/Access.kt` — parse `parvaz://<host>/<key>#<display-name>` → struct.
- `settings/ParvazSettings.kt` — EncryptedSharedPreferences for key; plain prefs for host + display name + language.
- Tests: Access parser (valid, missing key, missing host, with/without display name), round-trip storage.

## Milestone 12 — Onboarding (3 screens)

Farsi strings in `res/values/` (default locale). English in `res/values-en/`.

Screens, Compose:
1. `SplashScreen` — `پرواز` in Redaction + `شروع` rubber-stamp button.
2. `ImportAccessScreen` — single field + `چسباندن` + `اسکن QR` buttons. Auto-detects clipboard `parvaz://` on appear.
3. `VpnPermissionExplainerScreen` — Farsi explainer BEFORE the system prompt. One button → `VpnService.prepare(ctx)` intent.

Invoke `frontend-design:frontend-design` before each screen.

## Milestone 13 — Main screen (connected / disconnected)

`MainScreen` composable + `MainViewModel`:
- Disconnected: oxblood outlined `پرواز` rubber-stamp. Tap → connect.
- Connected: olive solid `در پرواز` stamp + `T+۰۰:۱۲:۴۷` uptime (Persian numerals via `java.text.NumberFormat` with `fa` locale).
- No other UI elements. Long-press for hidden settings sheet (language, access reset).
- State machine: Disconnected → Connecting → Connected → Disconnecting → Error.

## Milestone 14 — URL scheme handler + QR scanner

- `AndroidManifest.xml` — `<intent-filter>` for `parvaz://` scheme on MainActivity.
- QR scanner via `androidx.camera` + MLKit barcode (or zxing fallback).
- Both paths resolve to the same `ImportAccessScreen.onImport(Access)` callback.

## Milestone 15 — VpnService + tun2socks + sidecar

- `vpn/ParvazVpnService.kt` extends `VpnService`. Builds TUN (10.0.0.1/24, MTU 1500, routes 0.0.0.0/0).
- `vpn/CoreLauncher.kt` — `ProcessBuilder(nativeLibraryDir + "/libparvaz.so")`, pipes JSON config on stdin, reads `READY` line.
- `vpn/Tun2Socks.kt` — bundle `go-tun2socks` (Go AAR or a second sidecar) + wire TUN fd → SOCKS5 `127.0.0.1:1080`.
- Note `app/build.gradle.kts` already has `packaging.jniLibs.useLegacyPackaging = true`.

## Milestone 16 — Error / edge states (Farsi)

- `آدرس معتبر نیست — از فرستنده بخواهید دوباره بفرستد` — invalid access URL
- `اینترنت ندارید` — no network
- `سرور در دسترس نیست` — worker unreachable
- `دسترسی VPN رد شد` — user declined VpnService permission; Retry button
- All as diagonal oxblood stamp overlays when field-specific; else toast.

---

# Phase C — Integration

## Milestone 17 — Deploy worker + live E2E

- Deploy `worker/worker.js` to a test Cloudflare account via `wrangler`.
- Manual device smoke: install APK, paste parvaz:// URL, Connect, verify Instagram/Telegram load.
- Optional: `PARVAZ_E2E=1 go test -C core ./relay/...` against the live worker.

---

## Out of scope (explicit non-goals)

- **MITM / CA install** — not needed; TCP tunnel is opaque.
- **iOS** — different VPN model, different effort.
- **Play Store** — distribution via F-Droid + direct APK only.
- **Web dashboard, analytics, crash reporting** — zero-telemetry.
- **Standalone SOCKS5 daemon** — not a product, just a sidecar to the app.
