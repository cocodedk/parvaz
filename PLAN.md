# Implementation Plan — Parvaz

A Farsi-first Android app that tunnels **browser traffic** through a
user-deployed Google Apps Script relay, with SNI concealment + local
MITM. Architecturally aligned with MasterHttpRelayVPN-RUST. Three phases:
Go core, Android app, integration.

## Milestone 0 — Skeleton

- [x] `reference/` cloned (upstream Python, read-only)
- [x] `CLAUDE.md`, `PLAN.md`, `ARCHITECTURE.md` — project docs
- [x] `core/go.mod` — module `github.com/cocodedk/parvaz/core`
- [x] `.gitignore`, `LICENSE` (MIT), `version.txt`, GitHub scaffolding
- [x] Android Studio project scaffolded into `app/` with NOTAM theme
- [x] `git init` + `cocodedk/parvaz` public repo live

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
- [x] Builds cleanly for android/arm64 (~10 MB .so after M-mitm + M-dispatcher)
- [x] Real dispatcher wired; `stubDialer` removed. End-to-end TLS
      handshake through socks5 → dispatcher → interceptor is unit-tested.

## Milestone M-mitm · (NEXT, NEW)

**Target**: `core/mitm/{ca.go, leaf.go, interceptor.go}` + tests. Turns a
SOCKS5 CONNECT into an inspectable HTTP request the relay re-encodes as a
JSON envelope.

Design:
- CA at `<data-dir>/ca/ca.{crt,key}`; generated on first launch; Android reads PEM and triggers system CA-install intent.
- `interceptor.Intercept(ctx, rawConn, host, port)` replies SOCKS5 success then TLS-handshakes the client with a CA-signed leaf for `host`.
- Plaintext HTTP flows: each `http.Request` → `relay.Do` → response back through the TLS server conn.

Failing-test order: `TestCA_GenerateAndPersist`, `TestLeaf_SignedByCA_NameMatchesHost`, `TestInterceptor_TLSServer_AcceptsBrowserClientUsingCA`, `TestInterceptor_ForwardsHTTPRequestThroughRelay`, `TestInterceptor_GzipResponse_EchoedIntact`.

## Milestone M-dispatcher · DONE

- [x] `core/dispatcher/dispatcher.go` — routes per SOCKS5 CONNECT:
      - Path 1 (direct TCP) for `DefaultAllowList` — `*.google.com`,
        `*.googleusercontent.com`, `*.gstatic.com`, `*.googleapis.com`
      - Path 3 (MITM + Apps Script relay) — catch-all
- [x] Wildcard matching: exact + leading `*.` suffix, case-insensitive
- [x] `TestDispatcher_GoogleHost_UsesDirect`,
      `TestDispatcher_ArbitraryHost_UsesMITM`,
      `TestDispatcher_AllowListLookup_MatchesWildcards`

## Milestone M-sni-rewrite · DONE

- [x] `core/mitm/snitunnel.go` — terminate browser TLS with a CA-signed
      leaf, open upstream via fronter (SNI=www.google.com), pipe
      plaintext between the two TLS sessions. No Apps Script quota.
- [x] Dispatcher Path 2: `SNIRewriteList` + `SNITunneler` interface.
      `DefaultSNIRewriteList` = `*.youtube.com`, `*.ytimg.com`, `*.ggpht.com`
- [x] Fail-fast on misconfig: `SNIRewriteList` set with `SNITunnel` nil
      returns an error at `Dial`, not a silent fallback that'd burn
      Apps Script quota invisibly
- [x] TLS 1.2 floor enforced on fronter dialer (fixed as part of this
      milestone — applies to relay + SNI paths + any future fronted leg)

## Milestone 9b — parvazd wiring · DONE

- [x] `stubDialer` removed from `core/cmd/parvazd/main.go`
- [x] `buildPipeline(cfg, logger)` factored into `pipeline.go` — wires
      fronter → relay → CA → interceptor → dispatcher → socks5.Server
- [x] `--data-dir` flag (default `./parvaz-data`, resolved to absolute
      inside `buildPipeline` so CWD drift can't fork the CA)
- [x] `TestBuildPipeline_MITMHandshake` — SOCKS5 CONNECT through the
      real pipeline, TLS handshake succeeds against the generated CA

Go-side milestones are complete. Everything the sidecar needs is in
place. Next: **Phase B** — the Android side (VpnService wrapper,
tun2socks, sidecar launcher, CA-install flow, `parvaz://` intent
filter). See milestones 10–14 below.

# Phase B — Android App (Farsi-first)

## Milestone 10 — Compose NOTAM theme

- [x] Color.kt / Theme.kt / Type.kt — NOTAM palette, light-only
- [ ] Bundle **Vazirmatn** (required), Redaction, JetBrains Mono in `res/font/`
- [ ] Swap Type.kt from placeholder FontFamily.Serif/Monospace to bundled fonts
- [ ] Persian-aware letter-spacing (Vazirmatn = 0, Latin labels 2sp+)

## Milestone 10b — Brand identity (launcher + cold splash + in-app lockup)

Replace AGP-default icon + white system splash with NOTAM identity.

- [ ] Mark — single SVG master at `docs/identity/parvaz-mark.svg`; spec in `docs/identity.md`
- [ ] Adaptive launcher — vector `ic_launcher_background.xml` (Paper) + `_foreground.xml` (oxblood) + `monochrome` for Android 13+; delete legacy `mipmap-*dpi/ic_launcher*.webp`
- [ ] Cold splash — `androidx.core:core-splashscreen`; `Theme.Parvaz.Starting` with paper bg + `ic_splash_mark`; `installSplashScreen()` in `MainActivity.onCreate`
- [ ] In-app splash lockup (M12.1 extension) — mark above `پرواز` wordmark in `SplashScreen.kt`; preserve `شروع` CTA + test tag
- [ ] Web parity — same SVG in `website/`; set as GitHub social preview

## Milestone 11 — Settings + parvaz:// URL parser

- [x] `Access.kt` parses `parvaz://<deployment-id>/<key>#<display-name>`
- [x] `AccessParseException` with Farsi messages (9 unit tests)
- [ ] `ParvazSettings.kt` — EncryptedSharedPreferences for key; plain prefs for deployment ID, display name, language

## Milestone 12 — Onboarding (4 screens)

Farsi strings default (`res/values/`); English override (`res/values-en/`).

1. [x] M12.1 — `SplashScreen` — `پرواز` + `شروع` rubber-stamp button.
2. [x] M12.2 — `ImportAccessScreen` — single field + `چسباندن` + `اسکن QR`. Auto-detects clipboard `parvaz://` on appear.
3. [x] M12.3 — `CaInstallScreen` — Farsi walkthrough. `parvazd -gen-ca` writes the PEM under `filesDir/parvaz-data/ca/`; the screen pre-checks screen-lock via `KeyguardManager`, fires `ACTION_MANAGE_CA_CERTIFICATES`, then walks `AndroidCAStore` by SHA-256 fingerprint. State machine (GENERATING → READY → AWAITING_INSTALL → VERIFYING → INSTALLED/FAILED/NO_SCREEN_LOCK) survives rotation + process death via `rememberSaveable`.
4. [x] M12.4 — `VpnPermissionScreen` — Farsi explainer BEFORE Android's system VPN consent dialog. State machine (IDLE → AWAITING_SYSTEM_PROMPT → GRANTED/DENIED) rotation-safe via `rememberSaveable`. `Lifecycle.ON_RESUME` observer recovers from stuck AWAITING after process death / user-returned-without-responding.

## Milestone 13 — Main screen

- [x] **M13a** — Disconnected outline → tap → CONNECTING spinner →
      CONNECTED olive stamp + `T+HH:MM:SS` uptime (ticks from service's
      own `connectedAtMs` so recreation doesn't reset). Second tap
      disconnects. Persian numerals via `ui/util/PersianDigits`.
- [x] **M13b** — Long-press → `ModalBottomSheet`: language toggle + access
      reset (confirm dialog). SNI pool deferred.
- [ ] **M13c** — Service-binding refactor (with M15b tun2socks).
- [ ] **M13d** — Per-app locale via `LocaleManager.setApplicationLocales`.

## Milestone 14 — URL scheme handler + QR scanner

`parvaz://` intent-filter already lands on MainActivity; QR scanner via
`androidx.camera` + MLKit. Both paths feed `ImportAccessScreen.onImport`.

## Milestone 15 — VpnService + tun2socks + sidecar

- [x] **M15a** — VpnService + CoreLauncher.
- [x] **M15b-alpha** — `xjasonlyu/tun2socks/v2` in parvazd. Kotlin
      FD_CLOEXEC-clears (API 30+) and passes raw TUN fd via stdin.
      MITM uses `GetCertificate` so leaf matches browser SNI even on
      bare-IP targets. Own package in VpnService's disallowed-list.
- [x] **M15b-beta** — UDP/DNS. `core/socks5` speaks UDP ASSOCIATE;
      `core/doh` answers port-53 queries via RFC 8484 POST to
      `dns.google/dns-query`, transported through the Apps Script
      relay (not a direct fronter) — dns.google isn't served on
      Google's Apps edge, and riding the relay preserves DPI cover at
      the cost of ~1 Apps Script quota unit per lookup. TUN advertises
      `10.0.0.2` as DNS server. AAAA answered locally (TUN is v4-only);
      DoH failures synthesise SERVFAIL so resolvers bail fast. Non-DNS
      UDP is dropped (Chrome falls back to TCP/TLS). Proven live via
      `TestDNS_Live_ResolvesExampleComViaRelay` (2 A records in 1.95s).
- [ ] **M15c** — API ≤ 29 compat (currently hard-fails above minSdk 24).

## Milestone 16 — Error / edge states (Farsi)

Covers: bad parvaz:// URL, no internet, server unreachable, CA not
installed, VPN permission denied. Copy lives in `res/values/strings.xml`.

# Phase C — Integration

## Milestone 17 — Deploy Code.gs + live E2E

- Deploy `reference/apps_script/Code.gs` to a test Google account.
- Smoke: install APK on device, paste `parvaz://...`, install CA, Connect, load google.com + a non-Google site in Chrome.
- Optional gated test: `PARVAZ_E2E=1 go test -C core ./relay/...`.

## Out of scope (explicit non-goals)

- **Non-browser apps** (Instagram/Telegram/WhatsApp/banking native) — will fail by design (Android MITM limitation). Documented clearly.
- **iOS** — different VPN model.
- **Play Store** — F-Droid + sideload only.
- **Standalone SOCKS5 daemon** — just a sidecar to the app.
- **Analytics / crash reporting / telemetry** — zero-telemetry is a principle.
