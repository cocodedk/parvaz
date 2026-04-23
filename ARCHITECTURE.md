# Architecture — where Parvaz fits

Parvaz is an Android VPN app. A **Go sidecar** inside the APK tunnels raw
TCP bytes through a user-deployed **Cloudflare Worker** using a
domain-fronted WebSocket. This document shows the full data path and
where each component lives.

## The full data path

```
╔═══════════════════════════════════════════════════════════════════════════════╗
║                            ANDROID PHONE                                      ║
║                                                                               ║
║  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐                ║
║  │  Instagram app  │  │   Telegram app  │  │   Firefox app   │                ║
║  └────────┬────────┘  └────────┬────────┘  └────────┬────────┘                ║
║           │ TCP packets — apps route transparently through Parvaz VPN         ║
║           └───────────────────┬─┴────────────────────┘                        ║
║                               ▼                                               ║
║  ╔═══════════════════════════════════════════════════════════════════════╗   ║
║  ║                   THE PARVAZ APK — one install                        ║   ║
║  ║                                                                       ║   ║
║  ║  ┌─────────────────────────────────────────────────────────────────┐  ║   ║
║  ║  │  KOTLIN / COMPOSE  (app/)  Farsi-first NOTAM UI                 │  ║   ║
║  ║  │  · VpnService subclass: TUN on 10.0.0.1/24, routes 0.0.0.0/0    │  ║   ║
║  ║  │  · Sidecar launcher: ProcessBuilder("libparvaz.so") + stdin cfg │  ║   ║
║  ║  │  · tun2socks glue                                               │  ║   ║
║  ║  └────────────────────────────┬────────────────────────────────────┘  ║   ║
║  ║                               │ raw IP packets via TUN                ║   ║
║  ║                               ▼                                       ║   ║
║  ║  ┌─────────────────────────────────────────────────────────────────┐  ║   ║
║  ║  │  tun2socks  (bundled Go library)                                │  ║   ║
║  ║  │  Reassembles IP → TCP streams → SOCKS5 → 127.0.0.1:1080         │  ║   ║
║  ║  └────────────────────────────┬────────────────────────────────────┘  ║   ║
║  ║                               │ SOCKS5 CONNECT instagram.com:443     ║   ║
║  ║                               ▼                                       ║   ║
║  ║  ╔═══════════════════════════════════════════════════════════════╗    ║   ║
║  ║  ║  ⭐  PARVAZ SIDECAR  (core/)  —  Go binary libparvaz.so       ║    ║   ║
║  ║  ║     Listens on 127.0.0.1:1080 inside the app's process        ║    ║   ║
║  ║  ║                                                               ║    ║   ║
║  ║  ║  socks5/  → accepts CONNECT, calls relay.Dial(host, port)     ║    ║   ║
║  ║  ║         │                                                     ║    ║   ║
║  ║  ║         ▼                                                     ║    ║   ║
║  ║  ║  relay/   → opens a WebSocket to the configured Worker:       ║    ║   ║
║  ║  ║           wss://<worker>.workers.dev/tunnel                   ║    ║   ║
║  ║  ║             ?k=<access-key>                                   ║    ║   ║
║  ║  ║             &host=instagram.com&port=443                      ║    ║   ║
║  ║  ║         │                                                     ║    ║   ║
║  ║  ║         ▼                                                     ║    ║   ║
║  ║  ║  fronter/ — THE DOMAIN-FRONTING TRICK:                        ║    ║   ║
║  ║  ║   1. TCP connect to Cloudflare edge IP (e.g. 104.16.x.x)      ║    ║   ║
║  ║  ║   2. TLS handshake  SNI = <popular-cf-hosted-site>            ║    ║   ║
║  ║  ║      ↑ this is what the filter box sees ↑                     ║    ║   ║
║  ║  ║   3. Inside the TLS tunnel, send HTTP:                        ║    ║   ║
║  ║  ║      GET /tunnel?k=...&host=...&port=... HTTP/1.1             ║    ║   ║
║  ║  ║      Host: <worker>.workers.dev  ← real destination           ║    ║   ║
║  ║  ║      Upgrade: websocket                                       ║    ║   ║
║  ║  ╚═══════════════════════════╪═══════════════════════════════════╝    ║   ║
║  ╚══════════════════════════════╪═══════════════════════════════════════╝   ║
║                                 │                                            ║
╚═════════════════════════════════╪════════════════════════════════════════════╝
                                  │
                                  │ Encrypted HTTPS. Outside observer sees a
                                  │ TLS session to a popular Cloudflare-hosted
                                  │ site. Same IP. Same SNI. Same TLS fingerprint.
                                  │
                                  ▼
            ╔════════════════════════════════════════════════════╗
            ║        CLOUDFLARE EDGE LOAD BALANCER               ║
            ║ Terminates TLS. Reads Host header.                 ║
            ║ Host = <worker>.workers.dev → route to Workers.    ║
            ╚═══════════════════════╤════════════════════════════╝
                                    ▼
            ╔════════════════════════════════════════════════════╗
            ║     CLOUDFLARE WORKER   (on YOUR account)          ║
            ║     worker.js — ~50 lines:                         ║
            ║                                                    ║
            ║     import { connect } from "cloudflare:sockets";  ║
            ║     const sock = connect({                         ║
            ║         hostname: req.host, port: req.port         ║
            ║     });                                            ║
            ║     pipe WebSocket frames ⇆ socket bytes           ║
            ║                                                    ║
            ║  The Worker opens raw TCP from Cloudflare's edge   ║
            ║  to the real destination. Outbound IP is CF's —    ║
            ║  not blocked.                                      ║
            ╚═══════════════════════╤════════════════════════════╝
                                    ▼
                          ┌───────────────────┐
                          │  instagram.com    │
                          └─────────┬─────────┘
                                    ▼ response bytes travel back
                              (reverse chain: upstream TCP →
                               WebSocket → Parvaz sidecar →
                               tun2socks → TUN → Instagram app,
                               which thinks it had a normal TCP
                               chat with instagram.com)
```

## Who writes what

| Layer | Location | Language | Who |
|---|---|---|---|
| Farsi-first UI (splash, onboarding, main screen) | `app/presentation/` | Kotlin + Compose | **Us** |
| `VpnService` subclass + TUN routing | `app/vpn/` | Kotlin | **Us** |
| Main screen, settings, theme, sidecar launcher | `app/settings/`, `app/ui/` | Kotlin | **Us** |
| `tun2socks` (packet → TCP → SOCKS5) | `app/libs/` (bundled) | Go (OSS) | existing |
| **Parvaz sidecar** (socks5 + relay + fronter) | `core/` | Go | **Us** |
| **Cloudflare Worker** (TCP tunnel server) | `worker/worker.js` | JavaScript | **Us** |
| Android OS (`VpnService`, `ProcessBuilder`, `EncryptedSharedPreferences`) | — | — | Google |

## Core ↔ App boundary

The Go sidecar is compiled per Android ABI (`arm64-v8a`, `armeabi-v7a`,
`x86_64`, `x86`) and placed at `app/src/main/jniLibs/<abi>/libparvaz.so`.
AGP needs `packaging.jniLibs.useLegacyPackaging = true` so Android actually
extracts the file (without that, it's memory-mapped and `ProcessBuilder`
can't exec it). The Kotlin launcher:

1. Derives path from `ApplicationInfo.nativeLibraryDir + "/libparvaz.so"`.
2. `ProcessBuilder(path).redirectErrorStream(true).start()`.
3. Pipes `{worker_url, auth_key, listen_port, ...}` as JSON to stdin.
4. Reads `READY` on stdout.
5. From here the sidecar is just a SOCKS5 server on `127.0.0.1:<port>`.

**No JNI. No gomobile.** Process boundary + loopback socket. Side benefit: the same binary runs on any desktop OS for dev debugging (`go run ./cmd/parvazd`).

## Why Cloudflare Workers, not Apps Script

Parvaz originally planned to rewrite MasterHttpRelayVPN's Apps Script
backend. That path is blocked on Android:

- Apps Script's `UrlFetchApp.fetch()` is URL-based HTTP. It cannot tunnel
  opaque TLS bytes — which means HTTPS traffic requires MITM (MasterHttpRelayVPN's Python client does this with a generated CA).
- Android 7+ does not trust user-installed CAs for apps without per-app opt-in (`network-security-config`). Instagram, Telegram, WhatsApp, every banking app — all reject the MITM cert.
- So on Android the Apps Script path would break every app except maybe Firefox.

Cloudflare Workers' `cloudflare:sockets` API opens **raw outbound TCP
sockets**. We pipe the WebSocket binary stream straight to the destination's TCP socket. The bytes stay opaque. No MITM, no CA install, every app works.

## Why one repo

Parvaz is one product. Go sidecar, Kotlin app, JavaScript worker,
bilingual website — versions must stay aligned; a release ships all three
as one atomic artifact (one tag, one APK, one `wrangler deploy`).
