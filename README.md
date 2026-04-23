# Parvaz (پرواز)

*Persian for "flight" — a flight over the filter.*

An **Android VPN app** for Iranian users with no technical background.
A helper (relative, volunteer, activist) deploys a one-file **Cloudflare
Worker** and sends a `parvaz://` link via Telegram. The user scans a QR
code or pastes the link, taps one button, and all phone traffic tunnels
through the worker — invisible to filters that only see "traffic to
Cloudflare".

## Website

- [English](https://cocodedk.github.io/parvaz/)
- [فارسی (Persian)](https://cocodedk.github.io/parvaz/fa/)

## Status

**Pre-alpha — not implemented yet.** See [`PLAN.md`](./PLAN.md) for the
milestone list and [`ARCHITECTURE.md`](./ARCHITECTURE.md) for the full
data path.

## Download

Once the first release is cut:

```
https://github.com/cocodedk/parvaz/releases/latest/download/Parvaz.apk
```

F-Droid / sideload only. Google Play Store is not a viable distribution
channel for circumvention apps.

## What you need on the server side

A **Cloudflare Worker** deployed to your own Cloudflare account (free
tier is enough — 100K requests/day). See
[`worker/README.md`](./worker/README.md) for the 5-step deployment:

```sh
cd worker
npm install
npx wrangler deploy
```

The worker gives you a URL like `https://relay-iran.babak.workers.dev`.
Set an access key inside `worker.js`, then share the combined
`parvaz://<worker-host>/<key>` URL with users over Telegram.

## Build from Source

**Prerequisites:** Android Studio (latest), JDK 17, Go 1.24+.

```sh
git clone https://github.com/cocodedk/parvaz.git
cd parvaz

# Go sidecar (hermetic — works without Android)
go test -C core ./...                     # unit tests
go test -C core -race -cover ./...        # race + coverage

# Android app (requires Go sidecar compiled for target ABIs)
./gradlew test
./gradlew assembleDebug
./gradlew buildSmoke
```

Install git hooks once after cloning:

```sh
./scripts/install-hooks.sh
```

## Architecture

```
parvaz/
├── app/           Kotlin + Compose UI (Farsi-first), VpnService, tun2socks
├── core/          Go sidecar — fronter dialer + SOCKS5 + WebSocket relay
│   ├── fronter/   TLS-with-custom-SNI dialer + HTTP client
│   ├── relay/     WebSocket TCP tunnel to the Cloudflare Worker
│   ├── socks5/    local SOCKS5 listener
│   └── cmd/parvazd/  sidecar main, packaged as libparvaz.so
├── worker/        Cloudflare Worker (worker.js, wrangler.toml)
├── reference/     MasterHttpRelayVPN upstream — read-only historical
└── website/       Bilingual GitHub Pages (EN + FA)
```

See [`ARCHITECTURE.md`](./ARCHITECTURE.md) for the full data path,
layer rules, and the core↔app boundary.

## Tests

Go sidecar tests are hermetic — no Android, no Cloudflare required:

```sh
go test -C core ./...
```

Android tests:

```sh
./gradlew test            # JVM (domain layer)
./gradlew connectedCheck  # instrumented (emulator/device)
```

Live-network tests require a deployed Worker and are gated:

```sh
PARVAZ_E2E=1 go test -C core ./relay/...
```

## Why Cloudflare Workers?

Because the original Apps Script plan doesn't work on modern Android.
`UrlFetchApp.fetch()` is URL-based HTTP — it cannot tunnel opaque HTTPS
bytes without MITM, and Android 7+ won't trust user-installed CAs
without per-app opt-in. Cloudflare Workers have the
`cloudflare:sockets` API for raw outbound TCP — bytes pass through
transparently. See [`ARCHITECTURE.md`](./ARCHITECTURE.md) for details.

## Legal / ToS

Cloudflare Workers' acceptable-use policy is strict about proxy
services. Deploy to your **own** Cloudflare account only — do not
distribute shared deployments, do not commercialise. Personal,
research, and educational use only.

## Author

**Babak Bandpey** — [cocode.dk](https://cocode.dk) · [LinkedIn](https://linkedin.com/in/babakbandpey) · [GitHub](https://github.com/cocodedk)

## License

MIT | © 2026 [Cocode](https://cocode.dk) | Created by [Babak Bandpey](https://linkedin.com/in/babakbandpey)
