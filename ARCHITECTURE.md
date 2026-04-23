# Architecture — where Parvaz fits

Parvaz is an Android app that embeds a Go SOCKS5 core as a sidecar binary.
Monorepo, one APK, one release. The Kotlin side does UI + `VpnService` +
`tun2socks`. The Go side does the domain-fronting transport. This document
explains how the pieces connect.

## The full data path

```
╔═══════════════════════════════════════════════════════════════════════════════╗
║                            ANDROID PHONE                                      ║
║                                                                               ║
║  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐            ║
║  │  Instagram app  │    │   Firefox app   │    │   WhatsApp app  │            ║
║  └────────┬────────┘    └────────┬────────┘    └────────┬────────┘            ║
║           │                      │                      │                     ║
║           │  regular TCP/IP packets — apps don't know anything's happening    ║
║           └──────────────────────┼──────────────────────┘                     ║
║                                  │                                            ║
║                                  ▼                                            ║
║  ╔═══════════════════════════════════════════════════════════════════════╗   ║
║  ║                    THE PARVAZ APK — one install                       ║   ║
║  ║                                                                       ║   ║
║  ║  ┌─────────────────────────────────────────────────────────────────┐  ║   ║
║  ║  │  KOTLIN / COMPOSE  (app/)                                       │  ║   ║
║  ║  │  UI · settings storage · VpnService subclass · tun2socks glue   │  ║   ║
║  ║  └────────────────────────────┬────────────────────────────────────┘  ║   ║
║  ║                               │  raw IP packets via TUN (10.0.0.1/24) ║   ║
║  ║                               ▼                                       ║   ║
║  ║  ┌─────────────────────────────────────────────────────────────────┐  ║   ║
║  ║  │  tun2socks  (bundled Go library)                                │  ║   ║
║  ║  │  Reassembles IP packets → TCP flows → SOCKS5 to 127.0.0.1:1080  │  ║   ║
║  ║  └────────────────────────────┬────────────────────────────────────┘  ║   ║
║  ║                               │  SOCKS5 CONNECT instagram.com:443    ║   ║
║  ║                               │  + raw TLS bytes Instagram sent      ║   ║
║  ║                               ▼                                       ║   ║
║  ║  ╔═══════════════════════════════════════════════════════════════╗    ║   ║
║  ║  ║  ⭐  PARVAZ CORE  (core/)  —  Go sidecar binary               ║    ║   ║
║  ║  ║     Launched via ProcessBuilder("libparvaz.so")               ║    ║   ║
║  ║  ║     Listening on 127.0.0.1:1080 inside the app's process.     ║    ║   ║
║  ║  ║                                                               ║    ║   ║
║  ║  ║  socks5/  → relay/  → protocol/ (JSON envelope)               ║    ║   ║
║  ║  ║                                  │                            ║    ║   ║
║  ║  ║                                  ▼                            ║    ║   ║
║  ║  ║  fronter/ — THE DOMAIN-FRONTING TRICK:                        ║    ║   ║
║  ║  ║    1. TCP connect to 216.239.38.120:443  (a Google IP)        ║    ║   ║
║  ║  ║    2. TLS handshake  SNI = "www.google.com"                   ║    ║   ║
║  ║  ║       ↑ this is what the government DPI box sees ↑            ║    ║   ║
║  ║  ║    3. Inside the TLS tunnel, send:                            ║    ║   ║
║  ║  ║       POST /macros/s/<ID>/exec HTTP/1.1                       ║    ║   ║
║  ║  ║       Host: script.google.com  ← real destination             ║    ║   ║
║  ║  ║       Content-Type: application/json                          ║    ║   ║
║  ║  ║       <JSON envelope from protocol/>                          ║    ║   ║
║  ║  ╚═══════════════════════════╪═══════════════════════════════════╝    ║   ║
║  ╚══════════════════════════════╪═══════════════════════════════════════╝   ║
║                                 │                                            ║
╚═════════════════════════════════╪════════════════════════════════════════════╝
                                  │
                                  │  Encrypted HTTPS. From the outside (ISP,
                                  │  DPI, firewall) indistinguishable from
                                  │  someone opening www.google.com — same IP,
                                  │  SNI, TLS fingerprint, certificate.
                                  │
                                  ▼
            ╔════════════════════════════════════════════════════╗
            ║          GOOGLE'S FRONT-END LOAD BALANCER          ║
            ║  Decrypts TLS. Reads Host: header. Routes the      ║
            ║  request to script.google.com / Apps Script.       ║
            ╚═══════════════════════╤════════════════════════════╝
                                    ▼
            ╔════════════════════════════════════════════════════╗
            ║   APPS SCRIPT RUNTIME  (on YOUR Google account)    ║
            ║   Runs Code.gs (reference/apps_script/Code.gs):    ║
            ║     UrlFetchApp.fetch(req.u, {headers, method,…}); ║
            ║     return { s, h, b };                            ║
            ║   Outbound IP is Google's — not blocked.           ║
            ╚═══════════════════════╤════════════════════════════╝
                                    ▼
                          ┌───────────────────┐
                          │  instagram.com    │
                          └─────────┬─────────┘
                                    ▼  response bytes travel back
                              (reverse chain: Code.gs → TLS
                               tunnel → Parvaz core → tun2socks
                               → TUN → Instagram app, which
                               thinks it just had a normal TCP
                               chat with instagram.com)
```

## Who writes what

| Layer | Location | Language | Who |
|---|---|---|---|
| `VpnService` subclass + TUN routing | `app/vpn/` | Kotlin | **Us** |
| Main screen, settings, theme, sidecar launcher | `app/presentation/`, `app/settings/`, `app/ui/` | Kotlin | **Us** |
| `tun2socks` (packet→TCP→SOCKS5) | `app/libs/` (bundled) | Go (OSS) | existing library |
| **Parvaz core** (SOCKS5 + relay + envelope + fronter) | `core/` | Go | **Us** |
| Android OS APIs (`VpnService`, `ProcessBuilder`, `EncryptedSharedPreferences`) | — | Java/Kotlin | Google |
| `Code.gs` — server half | user's own Google account | Apps Script JS | Upstream `MasterHttpRelayVPN` |

## Core ↔ App boundary

The Go core is compiled per Android ABI (`arm64-v8a`, `armeabi-v7a`,
`x86_64`, `x86`) and placed at `app/src/main/jniLibs/<abi>/libparvaz.so`.
On Android the `PackageManager` auto-extracts native libraries into
`nativeLibraryDir`, so at runtime the Kotlin launcher calls
`ProcessBuilder` against that path, writes a JSON config to stdin,
and waits for a `READY` line. From there the core is a plain SOCKS5
server on `127.0.0.1:1080`.

**No JNI. No gomobile.** The boundary is a process boundary and a
loopback socket — which means the Go core is also trivially runnable
on a developer laptop (`go run ./cmd/parvazd`) for debugging.

## Why Go for the transport (not Kotlin)

- `crypto/tls.Config.ServerName` cleanly splits SNI from dial target in 3 lines; Android's Conscrypt-based stack requires a custom `SSLSocketFactory` and breaks across OS versions.
- HTTP/2 with custom ALPN and idle timeouts is idiomatic in `net/http`.
- gzip/brotli/zstd are all mature Go libs; zstd on JVM is painful.
- Upstream is Python — a near-1:1 port to Go, not a rewrite to Kotlin.
- Hermetic tests: `httptest.NewTLSServer` on any laptop — no Robolectric, no emulator.
- Every production circumvention app on Android (sing-box, Xray, V2Ray, Hiddify, NekoBox) uses Go for the transport layer. We're not inventing a pattern.

## Why one repo (not two)

Parvaz is a single product. The Go core is an implementation detail of the
app, not a separately-distributed tool. Monorepo keeps versions aligned, CI
simple, and releases atomic — one tag, one APK.
