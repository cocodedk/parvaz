# Implementation Plan — Parvaz

An Android VPN app (Kotlin + Compose) that embeds a Go SOCKS5 domain-fronting
core as a sidecar binary. Built in three phases: Go core first (testable
without Android), Android app second, live integration last.

---

## Milestone 0 — Skeleton

- [x] `reference/` cloned (upstream Python, read-only)
- [x] `.mcp.json` with claude-chat + mem0 entries
- [x] `CLAUDE.md`, `PLAN.md`, `ARCHITECTURE.md` — project docs
- [x] `core/go.mod` — module `github.com/cocodedk/parvaz/core`
- [x] `.gitignore`, `LICENSE` (MIT), `version.txt`, GitHub scaffolding
- [ ] Android Studio project scaffolded into `app/` (user, one-off)
- [ ] First `git init` + `gh repo create cocodedk/parvaz --public --push`

---

# Phase A — Go Core

No Android dependencies. Fully hermetic via `go test`.

## Milestone 1 — `core/protocol` envelope

Pure JSON encode/decode. Failing-test order:

1. `TestEncodeSingle_MinimalGET` — `{Method:"GET", URL:"..."}` → JSON with `{k, m, u, h, r}`; no `b`, no `ct` when body empty.
2. `TestEncodeSingle_POSTWithBody` — body base64 into `b`; `ct` set.
3. `TestEncodeSingle_HeaderFiltering` — drops `Host`, `Connection`, `Content-Length`, `Transfer-Encoding`, `Proxy-*`, `Priority`, `TE`.
4. `TestEncodeBatch` — `{Items:[...]}` → `{k, q:[...]}`; auth key top-level only.
5. `TestDecodeResponse_Success` — `{s,h,b}` → `Response{Status, Header, Body}` base64-decoded.
6. `TestDecodeResponse_Error` — `{e:"unauthorized"}` → typed error.
7. `TestDecodeBatchResponse_MixedErrors` — `{q:[{s,h,b},{e:...}]}` decoded in order with per-item errors preserved.

## Milestone 2 — `core/fronter` dialer

TLS-with-custom-SNI dialer via `net.Pipe()` + in-process TLS server.

1. `TestDial_UsesCustomSNI` — server records `ClientHelloInfo.ServerName`; assert `www.google.com` while dial target is `127.0.0.1:<port>`.
2. `TestDial_ReturnsConnError_WhenUnreachable`.
3. `TestDial_RespectsContextCancellation`.

## Milestone 3 — `core/fronter` client

Full HTTP/1.1 round-trip against `httptest.NewTLSServer`.

1. `TestClient_SendsHostHeaderOverridingSNI` — `Host:` is `script.google.com` while SNI is `www.google.com`.
2. `TestClient_POSTJSONBody_EchoedBack`.
3. `TestClient_HandlesNonSuccessStatus` — 503 returned verbatim.
4. `TestClient_PropagatesContextDeadline`.

HTTP/2 deferred to Milestone 7.

## Milestone 4 — `core/codec`

Table-driven tests.

1. `TestDecode_Identity` — passthrough.
2. `TestDecode_Gzip` — stdlib `compress/gzip`.
3. `TestDecode_Brotli` — `github.com/andybalholm/brotli`.
4. `TestDecode_Zstd` — `github.com/klauspost/compress/zstd`.
5. `TestDecode_UnknownEncoding_ReturnsError`.
6. `TestDecode_Chained_gzipThenBr`.

## Milestone 5 — `core/relay`

In-memory stub implements `Code.gs` semantics in `core/internal/testutil/stub.go`.

1. `TestRelay_GET_TunnelsThroughStub`.
2. `TestRelay_POST_BodyBase64RoundTrip`.
3. `TestRelay_HonorsContentEncoding` — stub returns gzip; relay decodes.
4. `TestRelay_UnauthorizedFromStub_ReturnsTypedError`.
5. `TestRelay_MultipleScriptIDs_RoundRobins`.

## Milestone 6 — `core/socks5`

Minimal SOCKS5 (no auth, CONNECT only).

1. `TestSOCKS5_NoAuth_Negotiation` — client offers `0x00`; server accepts.
2. `TestSOCKS5_CONNECT_ForwardsThroughRelay` — fake relay captures target host:port.
3. `TestSOCKS5_RejectsBIND`.
4. `TestSOCKS5_RejectsUDPAssociate`.
5. `TestSOCKS5_MalformedHandshake_ClosesConn`.

## Milestone 7 — HTTP/2 multiplexing (optimization)

Only after 1–6 green.

1. `TestH2_MultiplexesConcurrentRequests`.
2. `TestH2_ReconnectsAfterGOAWAY`.

## Milestone 8 — Request batching (optimization)

1. `TestBatcher_CoalescesWithinWindow`.
2. `TestBatcher_FlushesOnMaxBatchSize`.
3. `TestBatcher_FlushesOnContextCancel`.

## Milestone 9 — `core/cmd/parvazd` sidecar binary

Target-packaged as `libparvaz.so` per Android ABI. Flags mirror upstream `config.example.json`: `script_id(s)`, `auth_key`, `google_ip`, `front_domain`, `listen_host`, `listen_port`. Reads config via stdin JSON *or* command-line flags — the Kotlin app uses stdin for hygiene (no secrets in `/proc/<pid>/cmdline`).

- Smoke: binary starts, listens on `:1080`, prints `READY` to stdout. No unit test — Phase B's launcher covers it.

---

# Phase B — Android App

Depends on a compiled `libparvaz.so` in `app/src/main/jniLibs/<abi>/`.

## Milestone 10 — Compose theme (NOTAM)

- `ui/theme/Color.kt` — Paper/Ink/Oxblood/Burnt/Olive palette.
- `ui/theme/Type.kt` — Redaction (display), JetBrains Mono (body), Vazirmatn (Persian), Redaction 35 (stamps). Fonts in `res/font/`.
- `ui/theme/Theme.kt` — light `ColorScheme` (not dynamic).
- `ui/components/StampButton.kt` — rubber-stamp CTA composable.
- `ui/components/Cropmarks.kt` — 4 L-shaped corner marks.
- `ui/components/StatusPill.kt` — skew-rotated stamped status chip.

Invoke `frontend-design:frontend-design` before writing these.

## Milestone 11 — Settings storage

- `settings/ParvazSettings.kt` — wraps `SharedPreferences` (relay URL) + `EncryptedSharedPreferences` (access key).
- Tests: `ParvazSettingsTest` (Robolectric) — round-trip write/read, masked-on-read, clear-wipes-both.

## Milestone 12 — Main screen, disconnected

- `presentation/main/MainScreen.kt` + `MainViewModel.kt`.
- Two fields (Relay URL, Access Key) with paste-from-clipboard; Connect button disabled until both non-empty and URL parses.
- Inline validation: green check on valid `AKfycb...` deployment ID; red X otherwise.
- State: `MainUiState.Disconnected(relayUrl, accessKeyMasked)`.

## Milestone 13 — Sidecar launcher

- `app/build.gradle.kts` must set `android.packaging.jniLibs.useLegacyPackaging = true`. AGP 9 forbids the manifest attribute `android:extractNativeLibs="true"` and requires the Gradle-level opt-in. Since API 29 the default is memory-mapped-from-APK, which means `nativeLibraryDir` points to a virtual path `ProcessBuilder` cannot exec. The opt-in costs ~APK-size but is mandatory for the sidecar approach.
- `vpn/CoreLauncher.kt` — uses `ApplicationInfo.nativeLibraryDir` + `ProcessBuilder("libparvaz.so")`. Pipes JSON config to stdin. Reads first stdout line, expects `READY`. Health-checks SOCKS5 on `127.0.0.1:1080` via a trivial CONNECT probe before reporting success.
- Tests: `CoreLauncherTest` (instrumented) — start/stop/restart, config echoed back verbatim, health probe on port 1080.

## Milestone 14 — VpnService skeleton

- `vpn/ParvazVpnService.kt` extends `VpnService`. Builds a TUN with a private subnet (10.0.0.1/24), MTU 1500, and routes `0.0.0.0/0` into it.
- `presentation/main/VpnPermissionLauncher.kt` — handles `VpnService.prepare(context)` intent.
- Deferred: actual packet forwarding — that's M15.

## Milestone 15 — tun2socks integration

- Bundle `go-tun2socks` (or equivalent) as a second sidecar (or AAR). Wire TUN fd → tun2socks → `127.0.0.1:1080`.
- End-to-end manual smoke: with Apps Script stub running on a laptop (hot-spot or adb reverse), phone traffic routes through it.

## Milestone 16 — Main screen, connected

- Replace top half with live telemetry: uptime, bytes up/down, RTT to relay, active flows.
- Pull stats from Go core via a second port (`127.0.0.1:1081`, JSON `GET /stats`) — added to `core/cmd/parvazd`.
- Disconnect tears down VpnService + kills sidecar.

## Milestone 17 — Error states

- `REJECTED` oxblood stamp overlay when Apps Script returns `{"e":"unauthorized"}`.
- `UNREACHABLE` burnt-amber banner when TLS dial to Google fails.
- `VPN PERMISSION REQUIRED` with Retry when user declines the prepare dialog.

---

# Phase C — Integration

## Milestone 18 — Live E2E

- Deploy `Code.gs` to a test Google account.
- Manual on-device smoke: install APK, paste URL + key, Connect, confirm phone can reach a geo-fenced endpoint.
- Optional: `PARVAZ_E2E=1 go test -C core ./relay/...` hitting real Google from laptop.

---

## Out of scope (explicit non-goals)

- **Forking sing-box / writing a native outbound registration** — the Android app bundles its own transport; sing-box integration only if demand emerges after MVP.
- **MITM CA generation** — unnecessary; VpnService + SOCKS5 delivers whole TCP flows.
- **Play Store distribution** — violates Google's ToS; F-Droid / direct APK only.
- **Web dashboard, telemetry, crash reporting** — zero-telemetry is a design principle.
- **iOS port** — different VPN model, different effort.
