# Parvaz (پرواز)

*Persian for "flight" — a flight over the filter.*

An **Android VPN app** that routes your phone's traffic through a
Google Apps Script relay you deploy yourself, using domain fronting
(TLS SNI = `www.google.com`, HTTP `Host:` = `script.google.com`).

You install one APK, paste your Apps Script deployment URL + auth key,
tap **Connect**, and every app on the phone tunnels through your own
relay — invisible to DPI boxes that only see "traffic to Google".

Go rewrite of the client side of
[MasterHttpRelayVPN](https://github.com/masterking32/MasterHttpRelayVPN),
with an Android app built around it.

## Website

- [English](https://cocodedk.github.io/parvaz/)
- [فارسی (Persian)](https://cocodedk.github.io/parvaz/fa/)

## Status

**Pre-alpha — not implemented yet.** See [`PLAN.md`](./PLAN.md) for the
milestone list and [`ARCHITECTURE.md`](./ARCHITECTURE.md) for the
end-to-end data path.

## Download

Once the first release is cut:

```
https://github.com/cocodedk/parvaz/releases/latest/download/Parvaz.apk
```

F-Droid / sideload only. Google Play Store is not a viable distribution
channel for this kind of app.

## What you still need

A **deployed `Code.gs`** on your own Google account. See upstream
[`reference/apps_script/Code.gs`](./reference/apps_script/Code.gs) and
the README deployment steps. Parvaz replaces only the *client* half of
MasterHttpRelayVPN; the server half stays exactly as upstream ships it.

## Build from Source

**Prerequisites:** Android Studio (latest), JDK 17, Go 1.24+.

```sh
git clone https://github.com/cocodedk/parvaz.git
cd parvaz

# Go core (hermetic — works without Android)
go test -C core ./...                     # unit tests
go test -C core -race -cover ./...        # race + coverage

# Android app (requires Go core built for target ABIs)
./gradlew test                            # JVM unit tests
./gradlew assembleDebug                   # debug APK
./gradlew buildSmoke                      # CI smoke: debug + test + lint
```

Install git hooks once after cloning:

```sh
./scripts/install-hooks.sh
```

## Architecture

```
parvaz/
├── app/           Kotlin + Compose UI, VpnService, tun2socks, settings
├── core/          Go SOCKS5 + domain-fronting transport (sidecar binary)
│   ├── protocol/  JSON envelope encode/decode (pure, no network)
│   ├── fronter/   TLS-with-custom-SNI dialer + HTTP client
│   ├── codec/     gzip / br / zstd / identity decoders
│   ├── relay/     envelope + client + codec glue
│   ├── socks5/    local SOCKS5 listener
│   └── cmd/parvazd/  sidecar main, packaged as libparvaz.so
├── reference/     Upstream Python — read-only study material
└── website/       Bilingual GitHub Pages (EN + FA)
```

See [`ARCHITECTURE.md`](./ARCHITECTURE.md) for the full data path,
layer rules, and the core↔app boundary.

## Tests

Go core tests are hermetic — no Android required:

```sh
go test -C core ./...
```

Android tests:

```sh
./gradlew test            # JVM (domain layer)
./gradlew connectedCheck  # instrumented (needs emulator/device)
```

Live-network tests require a deployed `Code.gs` and are gated:

```sh
PARVAZ_E2E=1 go test -C core ./relay/...
```

## Legal / ToS

Google Apps Script's terms forbid this use. Deploy `Code.gs` to your
**own** Google account only — do not centralize deployments or
distribute a shared one. Personal, research, and educational use only.
See upstream disclaimer.

## Author

**Babak Bandpey** — [cocode.dk](https://cocode.dk) · [LinkedIn](https://linkedin.com/in/babakbandpey) · [GitHub](https://github.com/cocodedk)

## License

MIT | © 2026 [Cocode](https://cocode.dk) | Created by [Babak Bandpey](https://linkedin.com/in/babakbandpey)

Mirrors the upstream MIT license of MasterHttpRelayVPN.
