<p align="center">
  <img src="website/parvaz-mark.svg" alt="Parvaz" width="160"/>
</p>

# Parvaz (پرواز)

*Persian for "flight" — a flight over the filter.*

A Farsi-first **Android browser tunnel** for Iranian users. A technical
helper deploys a Google Apps Script one-file relay; a non-technical user
installs Parvaz, scans a QR from Telegram, installs a MITM certificate
via Android Settings (one-time), installs **Firefox Nightly** and flips
one `about:config` flag, then taps one button. **Stock Chrome and stock
Firefox ignore user-installed CAs on Android 7+; Firefox Nightly + the
flag is the only no-root browser that works (see scope below).**

Architecturally aligned with the proven
[MasterHttpRelayVPN-RUST](https://github.com/therealaleph/MasterHttpRelayVPN-RUST)
port. Parvaz's edge is the NOTAM visual identity, Farsi-by-default UI,
and tighter onboarding for non-technical users.

## Website

- [English](https://cocodedk.github.io/parvaz/)
- [فارسی (Persian)](https://cocodedk.github.io/parvaz/fa/)

## Status

**Pre-alpha — not implemented yet.** See [`PLAN.md`](./PLAN.md) for the
milestone list and [`ARCHITECTURE.md`](./ARCHITECTURE.md) for the full
data path.

## Honest scope

| | |
|---|---|
| ✅ **Firefox Nightly** + `enterprise_roots` flag | The supported browser. Tunnels arbitrary HTTPS through the on-device MITM. |
| ✅ Google-owned sites in any browser | YouTube, Google Search, etc. take the SNI-rewrite fast path — no MITM, no cert needed |
| ❌ Stock Chrome / Brave / Edge / Vivaldi / Bromite | Chromium ignores user CAs on Android 7+; no flag, no override |
| ❌ Stock Firefox / Firefox Beta / Firefox Focus | Firefox's NSS root store is baked into the APK; only Nightly's flag bridges it to Android's user CA store |
| ❌ Instagram / Telegram / WhatsApp / banking / streaming apps | Reject user-installed CAs by Android default — cert errors, no tunnel |
| ❌ Non-HTTP protocols (MTProto, SSH, raw TCP) | Apps Script can't tunnel raw TCP |

If you need native-app coverage or raw-TCP tunneling, pair Parvaz with a
local **xray** (or v2ray / sing-box) pointing at your own VPS —
documented approach from MasterHttpRelayVPN-RUST.

## First-time setup on the phone

After installing the Parvaz APK and tapping a `parvaz://` link, you do
three small things — once. Then you never touch any of this again.

### 1. Install the Parvaz certificate

Tap **Open Settings** in Parvaz. Parvaz drops `parvaz-ca.crt` into your
Downloads folder and opens Android Settings as close as it can to the
install screen. From there:

- **Samsung (One UI 6 / Galaxy):** Security and privacy → More security
  settings → Install from device storage → CA certificate →
  `parvaz-ca.crt` → tap **Install anyway**.
- **Pixel / stock Android (14, 15):** Security & privacy → More
  security & privacy → Encryption & credentials → Install a
  certificate → CA certificate → `parvaz-ca.crt` → **Install anyway**.

You'll be asked for your screen-lock PIN or fingerprint. Walk back to
Parvaz (gesture back); it auto-detects the cert and advances.

> Pre-condition: your phone must have a screen lock (PIN, pattern, or
> password) — Android refuses CA install otherwise.

### 2. Install Firefox Nightly

Stock browsers **ignore** user-installed certificates on Android 7+.
The only no-root browser that honors them is **Firefox Nightly**:

- **F-Droid:** search *Firefox Nightly*
- **Play Store:** search *Firefox Nightly for Developers*

### 3. Flip one flag in Firefox Nightly

Open Firefox Nightly → paste `about:config` in the URL bar → accept
the warning → search `security.enterprise_roots.enabled` → tap to
toggle to **true** → fully close and reopen Firefox Nightly.

That's it. Browse normally in Firefox Nightly; HTTPS pages route
through Parvaz. Other browsers will still load Google-owned sites
(YouTube, Search, Maps) via the SNI-rewrite fast path with no cert
needed; everything else needs Firefox Nightly.

## Download

Once the first release is cut:

```
https://github.com/cocodedk/parvaz/releases/latest/download/Parvaz.apk
```

F-Droid / sideload only. Google Play Store is not a viable channel.

## What you need on the server side

A **Google Apps Script** deployment (free tier — 20k requests/day per
deployment). One-time `Code.gs` deploy on your own Google account:

1. Open https://script.google.com → New project.
2. Paste [`reference/apps_script/Code.gs`](./reference/apps_script/Code.gs) (or the identical file from upstream).
3. Change `AUTH_KEY` to a strong random string.
4. Deploy → New deployment → Web app → Execute as Me → Anyone.
5. Copy the deployment URL — extract the `AKfycby...` segment.

Share the combined `parvaz://<deployment-id>/<access-key>` URL with
users over Telegram (or a QR code).

## Build from Source

**Prerequisites:** Android Studio (latest), JDK 17, Go 1.24+.

```sh
git clone https://github.com/cocodedk/parvaz.git
cd parvaz

go test -C core ./...                     # hermetic unit tests
go test -C core -race -cover ./...

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
├── app/           Kotlin + Compose (Farsi-first), VpnService, tun2socks
├── core/          Go sidecar
│   ├── fronter/   TLS-with-custom-SNI dialer + HTTP client
│   ├── protocol/  Apps Script envelope encode/decode
│   ├── codec/     gzip / br / zstd decoders
│   ├── relay/     envelope + fronted client glue
│   ├── socks5/    local SOCKS5 listener
│   ├── mitm/      (next milestone) local TLS MITM
│   ├── dispatcher/ (next milestone) Google-allowlist vs MITM+relay
│   └── cmd/parvazd/ sidecar main → libparvaz.so
├── reference/     Upstream MasterHttpRelayVPN Python — read-only
└── website/       Bilingual GitHub Pages (EN + FA)
```

See [`ARCHITECTURE.md`](./ARCHITECTURE.md) for the full data path.

## Tests

Go sidecar tests are hermetic — no Android, no Google required:

```sh
go test -C core ./...
```

Android tests:

```sh
./gradlew test            # JVM (domain layer)
./gradlew connectedCheck  # instrumented (emulator/device)
```

## Alternative: use MasterHttpRelayVPN-RUST directly

If you don't need the Parvaz UX touches — Farsi-first, NOTAM aesthetic,
tight onboarding — [MasterHttpRelayVPN-RUST](https://github.com/therealaleph/MasterHttpRelayVPN-RUST)
ships today with prebuilt APKs. It has the same architecture and a full
English + Persian walkthrough. Parvaz only makes sense vs. that project
if the UX differentiation matters to you.

## Legal / ToS

Google Apps Script's terms forbid this use. Deploy `Code.gs` to your
**own** Google account only. Personal, research, and educational use
only. See upstream disclaimer.

## Author

**Babak Bandpey** — [cocode.dk](https://cocode.dk) · [LinkedIn](https://linkedin.com/in/babakbandpey) · [GitHub](https://github.com/cocodedk)

## License

MIT | © 2026 [Cocode](https://cocode.dk) | Created by [Babak Bandpey](https://linkedin.com/in/babakbandpey)
