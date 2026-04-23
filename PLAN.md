# Implementation Plan — Parvaz

A Farsi-first Android app that tunnels **browser traffic** through a
user-deployed Google Apps Script relay, with SNI concealment + local
MITM. Architecturally aligned with MasterHttpRelayVPN-RUST. Three phases:
Go core, Android app, integration.

---

## Milestone 0 — Skeleton

- [x] `reference/` cloned (upstream Python, read-only)
- [x] `CLAUDE.md`, `PLAN.md`, `ARCHITECTURE.md` — project docs
- [x] `core/go.mod` — module `github.com/cocodedk/parvaz/core`
- [x] `.gitignore`, `LICENSE` (MIT), `version.txt`, GitHub scaffolding
- [x] Android Studio project scaffolded into `app/` with NOTAM theme
- [x] `git init` + `cocodedk/parvaz` public repo live

---

# Phase A — Go Core

## Milestone 1 — `core/protocol` envelope

- [x] Request, BatchRequest, Response, BatchResponse, ServerError
- [x] EncodeSingle / EncodeBatch / DecodeResponse / DecodeBatchResponse
- [x] 7 tests · 81% coverage

## Milestone 2 — `core/fronter` dialer

- [x] `Dialer{FrontDomain, InsecureSkipVerify, BaseDialer, TLSConfigHook}`
- [x] 3 tests · 87.5% coverage

## Milestone 3 — `core/fronter` HTTP client

- [x] `NewHTTPClient(d, target) → *http.Client`
- [x] 4 tests — SNI/Host split, POST echo, 503, context deadline

## Milestone 4 — `core/codec`

- [x] identity · gzip · brotli · zstd · chained — 6 tests · 94% cov

## Milestone 5 — `core/relay`

- [x] Config{HTTPClient, ScriptURLs, AuthKey} → Do(ctx, req) → Response
- [x] Multiple ScriptURLs round-robin; Content-Encoding auto-decoded
- [x] 5 tests · 76.5% cov · in-process `testutil.AppsScriptStub`

## Milestone 6 — `core/socks5` listener

- [x] No auth, CONNECT-only; rejects BIND + UDP ASSOCIATE
- [x] 5 tests · 69.5% cov

## Milestone 9 — `core/cmd/parvazd` sidecar

- [x] Flags + stdin JSON config merge; prints `READY`; listens on :1080
- [x] Builds cleanly for android/arm64 (~5.4 MB .so)
- [ ] Wire real dispatcher once M-mitm + M-dispatcher land (currently stubDialer)

## Milestone M-mitm · (NEXT, NEW)

**Target**: `core/mitm/{ca.go, leaf.go, interceptor.go}` + tests.

The piece that turns a SOCKS5 CONNECT into an inspectable HTTP request
which the relay can re-encode as a JSON envelope.

Design:
- CA persisted at `<data-dir>/ca/ca.crt` + `ca/ca.key`. Generate on first
  launch. Android app reads the PEM and triggers the system CA-install
  intent.
- `interceptor.Intercept(ctx, rawConn, host, port)` accepts a SOCKS5-era
  TCP conn, sends back a SOCKS5 "CONNECT succeeded" reply, performs a
  TLS handshake WITH the client using a leaf cert signed by our CA,
  named for `host`.
- Once the plaintext HTTP flows, each `http.Request` → `relay.Do` →
  `http.Response` → written back through the TLS server conn.

Failing-test order:
1. `TestCA_GenerateAndPersist` — create, reload from disk, PEM-decode.
2. `TestLeaf_SignedByCA_NameMatchesHost` — x509.Verify succeeds.
3. `TestInterceptor_TLSServer_AcceptsBrowserClientUsingCA` — in-process client trusts the CA, performs handshake, sees our cert for `example.com`.
4. `TestInterceptor_ForwardsHTTPRequestThroughRelay` — stub relay captures the request; cert + body + method round-trip.
5. `TestInterceptor_GzipResponse_EchoedIntact` — end-to-end through codec.

## Milestone M-dispatcher · (NEW)

**Target**: `core/dispatcher/dispatcher.go` + tests.

Per SOCKS5 CONNECT, choose:
- **SNI-rewrite tunnel** — target host is in a hardcoded allow-list of Google-owned domains. Open a direct TCP stream via `fronter.Dialer` with SNI = `www.google.com`, Host (when HTTP) = target. Pipe bytes back.
- **MITM + Apps Script relay** — default for everything else. Delegate to `mitm.Interceptor`.

Allow-list (match mhrv-rs): `*.google.com`, `*.googleusercontent.com`, `*.gstatic.com`, `*.googleapis.com`, `*.youtube.com`, `*.ytimg.com`, `*.ggpht.com`. Exact matches + wildcard suffix.

Failing-test order:
1. `TestDispatcher_GoogleHost_UsesSNIRewrite`
2. `TestDispatcher_ArbitraryHost_UsesMITM`
3. `TestDispatcher_AllowListLookup_MatchesWildcards`

## Milestone 9b — parvazd wiring

Replace the stubDialer in `core/cmd/parvazd/main.go` with
`dispatcher.New(mitmInterceptor, relayClient, allowList).Dial` so the
SOCKS5 server actually forwards traffic.

---

# Phase B — Android App (Farsi-first)

## Milestone 10 — Compose NOTAM theme

- [x] Color.kt / Theme.kt / Type.kt — NOTAM palette, light-only
- [ ] Bundle **Vazirmatn** (required), Redaction, JetBrains Mono in `res/font/`
- [ ] Swap Type.kt from placeholder FontFamily.Serif/Monospace to bundled fonts
- [ ] Persian-aware letter-spacing (Vazirmatn = 0, Latin labels 2sp+)

## Milestone 11 — Settings + parvaz:// URL parser

- [x] `Access.kt` parses `parvaz://<deployment-id>/<key>#<display-name>`
- [x] `AccessParseException` with Farsi messages (9 unit tests)
- [ ] `ParvazSettings.kt` — EncryptedSharedPreferences for key; plain prefs for deployment ID, display name, language

## Milestone 12 — Onboarding (4 screens)

Farsi strings default (`res/values/`); English override (`res/values-en/`).

1. `SplashScreen` — `پرواز` + `شروع` rubber-stamp button.
2. `ImportAccessScreen` — single field + `چسباندن` + `اسکن QR`. Auto-detects clipboard `parvaz://` on appear.
3. `CaInstallScreen` — Farsi walkthrough. Export PEM, trigger Android CA-install intent, verify via `AndroidCAStore` fingerprint check.
4. `VpnPermissionExplainerScreen` — Farsi explainer BEFORE the system prompt.

## Milestone 13 — Main screen

- Disconnected: oxblood outline `پرواز` stamp. Tap → connect state machine.
- Connected: olive solid `در پرواز` + `T+۰۰:۱۲:۴۷` uptime in Persian numerals.
- Long-press for hidden settings sheet (language, access reset, SNI pool).

## Milestone 14 — URL scheme handler + QR scanner

- `AndroidManifest.xml` — `<intent-filter>` for `parvaz://` on MainActivity.
- QR scanner via `androidx.camera` + MLKit barcode.
- Both paths resolve to the same `ImportAccessScreen.onImport(Access)`.

## Milestone 15 — VpnService + tun2socks + sidecar

- `vpn/ParvazVpnService.kt` — TUN 10.0.0.1/24, MTU 1500, routes 0.0.0.0/0.
- `vpn/CoreLauncher.kt` — `ProcessBuilder(nativeLibraryDir + "/libparvaz.so")`, stdin JSON, reads `READY`.
- `vpn/Tun2Socks.kt` — gomobile AAR of a minimal tun2socks OR sing-box subset. Wire TUN fd → SOCKS5 `127.0.0.1:1080`.
- `app/build.gradle.kts` already has `packaging.jniLibs.useLegacyPackaging = true`.

## Milestone 16 — Error / edge states (Farsi)

- `آدرس معتبر نیست — از فرستنده بخواهید دوباره بفرستد`
- `اینترنت ندارید`
- `سرور در دسترس نیست`
- `گواهی نصب نشده است — دوباره تلاش کنید`
- `دسترسی VPN رد شد`

---

# Phase C — Integration

## Milestone 17 — Deploy Code.gs + live E2E

- Deploy `reference/apps_script/Code.gs` to a test Google account.
- Smoke: install APK on device, paste `parvaz://...`, install CA, Connect, load google.com + a non-Google site in Chrome.
- Optional gated test: `PARVAZ_E2E=1 go test -C core ./relay/...`.

---

## Out of scope (explicit non-goals)

- **Non-browser apps** (Instagram/Telegram/WhatsApp/banking native) — will fail by design (Android MITM limitation). Documented clearly.
- **iOS** — different VPN model.
- **Play Store** — F-Droid + sideload only.
- **Standalone SOCKS5 daemon** — just a sidecar to the app.
- **Analytics / crash reporting / telemetry** — zero-telemetry is a principle.
