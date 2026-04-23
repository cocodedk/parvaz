# Architecture — where Parvaz fits

Parvaz is a Farsi-first Android app that tunnels **browser traffic**
through a Google Apps Script relay deployed by a technical helper. The
architecture matches the proven [MasterHttpRelayVPN-RUST](https://github.com/therealaleph/MasterHttpRelayVPN-RUST)
port; Parvaz's edge is the NOTAM aesthetic + Farsi-by-default UI + tighter
onboarding.

## The full data path

```
╔═══════════════════════════════════════════════════════════════════════════════╗
║                            ANDROID PHONE                                      ║
║                                                                               ║
║  ┌─────────────────┐  ┌─────────────────┐                                     ║
║  │  Chrome / Firefox app (browser traffic)                │                   ║
║  └────────┬────────┘  └────────┬────────┘                                     ║
║           │ TCP packets — captured transparently via VpnService               ║
║           └───────────────────┬┴────────────────────┘                         ║
║                               ▼                                               ║
║  ╔═══════════════════════════════════════════════════════════════════════╗   ║
║  ║                   THE PARVAZ APK — one install                        ║   ║
║  ║                                                                       ║   ║
║  ║  ┌─────────────────────────────────────────────────────────────────┐  ║   ║
║  ║  │  KOTLIN / COMPOSE  Farsi-first NOTAM UI                         │  ║   ║
║  ║  │  · VpnService: TUN 10.0.0.1/24, routes 0.0.0.0/0                │  ║   ║
║  ║  │  · Sidecar launcher: ProcessBuilder("libparvaz.so") + stdin cfg │  ║   ║
║  ║  │  · MITM CA install flow (Android Settings → CA certificate)     │  ║   ║
║  ║  └────────────────────────────┬────────────────────────────────────┘  ║   ║
║  ║                               ▼                                       ║   ║
║  ║  ┌─────────────────────────────────────────────────────────────────┐  ║   ║
║  ║  │  tun2socks  (bundled)                                           │  ║   ║
║  ║  │  IP packets → TCP flows → SOCKS5 → 127.0.0.1:1080               │  ║   ║
║  ║  └────────────────────────────┬────────────────────────────────────┘  ║   ║
║  ║                               ▼                                       ║   ║
║  ║  ╔═══════════════════════════════════════════════════════════════╗    ║   ║
║  ║  ║  ⭐  PARVAZ SIDECAR  (core/)  —  libparvaz.so                 ║    ║   ║
║  ║  ║                                                               ║    ║   ║
║  ║  ║  socks5/    → accepts CONNECT host:port                       ║    ║   ║
║  ║  ║        │                                                      ║    ║   ║
║  ║  ║        ▼                                                      ║    ║   ║
║  ║  ║  dispatcher/  ──────┬── *.google.com / *.youtube.com etc. ──► ║    ║   ║
║  ║  ║                     │                                         ║    ║   ║
║  ║  ║                     │    SNI-rewrite tunnel (no MITM, no      ║    ║   ║
║  ║  ║                     │    relay; direct TCP via fronter)       ║    ║   ║
║  ║  ║                     │                                         ║    ║   ║
║  ║  ║                     └── anything else:                        ║    ║   ║
║  ║  ║                                                               ║    ║   ║
║  ║  ║  mitm/      → TLS server presents a leaf cert signed by our   ║    ║   ║
║  ║  ║               on-device CA. Client (Chrome) accepts (because  ║    ║   ║
║  ║  ║               user installed the CA in Settings). TLS ends    ║    ║   ║
║  ║  ║               locally — each HTTP request becomes inspectable ║    ║   ║
║  ║  ║        │                                                      ║    ║   ║
║  ║  ║        ▼                                                      ║    ║   ║
║  ║  ║  relay/     → wraps the decoded request in the Apps Script    ║    ║   ║
║  ║  ║               envelope: {k, m, u, h, b?, ct?, r}              ║    ║   ║
║  ║  ║        │                                                      ║    ║   ║
║  ║  ║        ▼                                                      ║    ║   ║
║  ║  ║  fronter/   → TCP connect <google_ip>:443                     ║    ║   ║
║  ║  ║               TLS handshake with SNI = www.google.com         ║    ║   ║
║  ║  ║               HTTP Host: script.google.com                    ║    ║   ║
║  ║  ║               POST /macros/s/<DEPLOYMENT_ID>/exec             ║    ║   ║
║  ║  ╚═════════════════════════════╪═══════════════════════════════╝      ║   ║
║  ╚════════════════════════════════╪═══════════════════════════════════════╝   ║
║                                   │                                           ║
╚═══════════════════════════════════╪═══════════════════════════════════════════╝
                                    │
                                    │ HTTPS. DPI sees www.google.com — same IP,
                                    │ SNI, TLS fingerprint as a real google.com
                                    │ session.
                                    ▼
            ╔════════════════════════════════════════════════════╗
            ║         GOOGLE EDGE FRONTEND (:443)                ║
            ║ Decrypts TLS. Reads Host header.                   ║
            ║ Host = script.google.com → route to Apps Script.   ║
            ╚═══════════════════════╤════════════════════════════╝
                                    ▼
            ╔════════════════════════════════════════════════════╗
            ║      APPS SCRIPT RUNTIME   (on YOUR Google account)║
            ║      Code.gs (reference/apps_script/Code.gs):      ║
            ║        var res = UrlFetchApp.fetch(req.u, {        ║
            ║            method: req.m, headers: req.h,          ║
            ║            payload: decode64(req.b),               ║
            ║        });                                         ║
            ║        return { s, h, b };                         ║
            ║                                                    ║
            ║   Apps Script calls the real target from inside    ║
            ║   Google's datacenter — the origin sees a Google   ║
            ║   IP + `Google-Apps-Script` User-Agent.            ║
            ╚═══════════════════════╤════════════════════════════╝
                                    ▼
                          ┌───────────────────┐
                          │  news-site.com    │
                          └─────────┬─────────┘
                                    ▼
                           (response envelope travels back
                            up the chain, Parvaz re-encrypts
                            with the leaf cert, Chrome sees
                            a "normal" HTTPS response)
```

## Why browsers only — the MITM honesty

MITM works because **we control the CA the user installed** and present
a leaf cert for whichever host the browser requests. Chrome and Firefox
on Android honor user-installed CAs in the system store (with explicit
opt-in on Android 7+ via our CA-install flow).

**Instagram / Telegram / WhatsApp / banking apps** set
`networkSecurityConfig` to trust **system CAs only** — they reject our
leaf. Their TLS handshake fails with cert validation errors. This is a
deliberate Google policy decision (Android 7, API 24+ default) and there
is no workaround short of rooting the device.

So Parvaz's honest scope is: **browser traffic, plus Google-owned
domains via SNI-rewrite** (which needs no MITM). Exactly what
MasterHttpRelayVPN-RUST ships.

## Who writes what

| Layer | Location | Language | Who |
|---|---|---|---|
| Farsi-first UI | `app/presentation/` | Kotlin + Compose | **Us** |
| `VpnService` + TUN routing | `app/vpn/` | Kotlin | **Us** |
| MITM CA install UI + fingerprint verify | `app/mitm/` | Kotlin | **Us** |
| `tun2socks` (IP → SOCKS5) | bundled | Go (OSS) | existing |
| **Parvaz sidecar** (socks5 + dispatcher + mitm + relay + fronter) | `core/` | Go | **Us** |
| `Code.gs` (Apps Script server) | user's Google account | JS (Apps Script) | Upstream MasterHttpRelayVPN — unchanged |

## Core ↔ App boundary

Go sidecar cross-compiled per ABI into `app/src/main/jniLibs/<abi>/libparvaz.so`.
AGP needs `packaging.jniLibs.useLegacyPackaging = true` so the `.so`
hits disk (needed for `ProcessBuilder` to exec it). Kotlin launcher:

1. `ApplicationInfo.nativeLibraryDir + "/libparvaz.so"`.
2. `ProcessBuilder(path).redirectErrorStream(true).start()`.
3. Pipe JSON config on stdin (deployment URL, access key, CA key paths, listen port).
4. Read `READY` on stdout.
5. Sidecar is now a SOCKS5 server on `127.0.0.1:<port>`.

**No JNI. No gomobile.** Process boundary + loopback socket + shared
filesystem for the CA key. Same binary runs on any desktop OS for
debugging.

## Why one repo

Go sidecar + Kotlin app + bilingual website ship together. Versions
must stay aligned; a release is one tag, one APK, one `Code.gs` drop-in.
