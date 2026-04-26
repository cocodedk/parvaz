<p align="center">
  <img src="website/parvaz-mark.svg" alt="Parvaz" width="160"/>
</p>

<p align="center"><sub>— Welcome aboard · <strong>cocode.dk Airways</strong> —</sub></p>

# Parvaz (پرواز)

*Persian for "flight" — a flight over the filter.*

A Farsi-first **Android browser tunnel** for Iranian users. A technical
helper deploys a Google Apps Script one-file relay and shares the
access link over a **secure messenger (Signal or Telegram — not
WhatsApp)**; the user installs Parvaz, scans a QR or taps the
`parvaz://` link, installs a MITM certificate via Android Settings
(one-time), then taps one button and browses normally in **Chrome**
(or any Chromium browser — Brave, Edge, Vivaldi). **Chrome trusts the
user-installed Parvaz CA out of the box on Android — no flags, no
about:config, no root.** Non-browser apps (Instagram, Telegram native,
banking, streaming) are out of scope by Android design — see honest
scope below.

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

## 🚨 Trust warning — read this first

**The relay operator can read every cookie, password, and message you
send through the tunnel — in plaintext.**

Parvaz works by terminating TLS locally with a MITM certificate, then
re-encapsulating the request inside an Apps Script JSON envelope
(`{ k, m, u, h, b }` — key, method, URL, **headers including
`Cookie` / `Authorization`**, **body including form fields with
passwords**). The Apps Script runtime owned by your relay operator
calls `UrlFetchApp.fetch(target)` with that data. Both they and Google
can log everything end-to-end, including `Set-Cookie` in responses.

This is a **fundamental property** of MITM-via-relay architecture, not
a bug, not something a future release can fix while keeping the
domain-fronting design.

> **Do NOT sign in to email, banking, social, or any service through
> Parvaz unless you trust the relay operator 100 % — not 99.99 %.**
> Use this tunnel only for read-only browsing, public information, or
> with throwaway accounts you do not care about losing.

If the operator is **you** (you deployed your own `Code.gs`) the only
attack surface is Google's own logging. If the operator is **someone
else**, every credential you submit is theirs.

## Honest scope

| | |
|---|---|
| ✅ **Chrome** (and other Chromium: Brave, Edge, Vivaldi) | **Recommended.** Trusts the user-installed CA with no setup; Chrome's Certificate Transparency enforcement has an explicit carve-out for chains rooted in a user CA, so the MITM is transparent. |
| ✅ Google-owned sites in any browser | YouTube, Google Search, fonts.googleapis.com, etc. take the SNI-rewrite fast path — no MITM, no cert trust needed |
| ⚠️ Firefox Nightly + `security.enterprise_roots.enabled` | Possible but painful. Stable/Beta hide `about:config` so the flag can't be set there at all; Nightly resets the flag to `false` on every restart (Mozilla bug [fenix#18990](https://github.com/mozilla-mobile/fenix/issues/18990) — open since 2021, never fixed). **Use Chrome instead.** |
| ❌ DuckDuckGo browser | Confirmed broken. Chromium-based but configured more strictly — rejects the Parvaz CA |
| ❌ Stock Firefox / Firefox Beta / Firefox Focus | No `about:config` UI on these channels — there is no way to flip the trust flag |
| ❌ Instagram / Telegram / WhatsApp / banking / streaming apps | Reject user-installed CAs by Android default — cert errors, no tunnel |
| ❌ Non-HTTP protocols (MTProto, SSH, raw TCP) | Apps Script can't tunnel raw TCP |
| ⚠️ Bandwidth and WebSockets | Apps Script bottleneck: throughput around ~5 KB/s observed; WebSockets do not establish through the relay |

If you need native-app coverage or raw-TCP tunneling, pair Parvaz with a
local **xray** (or v2ray / sing-box) pointing at your own VPS —
documented approach from MasterHttpRelayVPN-RUST.

## First-time setup on the phone

After installing the Parvaz APK and tapping a `parvaz://` link, you do
two small things — once. Then you never touch any of this again.

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

### 2. Tap Connect, then open Chrome

Tap the rubber-stamp button on Parvaz's main screen. When it flips to
**در پرواز** (in flight), open **Chrome** and browse normally — HTTPS
pages route through Parvaz with no further configuration. Brave, Edge,
and Vivaldi work the same way; the Parvaz CA you just installed is the
only piece they need.

> Throughput is currently around ~5 KB/s and WebSockets do not work —
> these are limits of the underlying Apps Script relay, not the
> browser. Plain HTTPS pages load slowly but reliably.

## Download

[**Download Parvaz.apk (latest)**](https://github.com/cocodedk/parvaz/releases/latest/download/Parvaz.apk)
· [SHA-256](https://github.com/cocodedk/parvaz/releases/latest/download/Parvaz.apk.sha256)
· [All releases](https://github.com/cocodedk/parvaz/releases)

CLI install + integrity check (recommended on hostile networks):

```sh
curl -LO https://github.com/cocodedk/parvaz/releases/latest/download/Parvaz.apk
curl -LO https://github.com/cocodedk/parvaz/releases/latest/download/Parvaz.apk.sha256
sha256sum -c Parvaz.apk.sha256        # must print: Parvaz.apk: OK
adb install Parvaz.apk                 # or sideload through your file manager
```

Sideload / F-Droid only — Google Play Store is not a viable channel.

## Helper guide — deploy the relay on Apps Script

You (the helper) need **a free Google account**. The end user never
opens script.google.com. Total time: ~5 minutes, once. Free-tier
quota: ~20k requests/day per deployment.

### 1. Create the Apps Script project

1. Open <https://script.google.com> → click **New project**.
2. Delete the default `Code.gs` contents.
3. Open
   [`reference/apps_script/Code.gs`](./reference/apps_script/Code.gs)
   in the Parvaz repo, copy the entire file, paste into the editor.

### 2. Set a strong AUTH\_KEY

Generate a long random key — do not reuse a password:

```sh
openssl rand -base64 32
# example output: 7dF9KmY3pQ8xV2nR5tL1aB4cE6gH9jM=
```

In `Code.gs`, replace the placeholder:

```js
var AUTH_KEY = "7dF9KmY3pQ8xV2nR5tL1aB4cE6gH9jM=";  // your value
```

Save: **File → Save** (`Ctrl/Cmd+S`).

### 3. Deploy as a Web App

1. **Deploy → New deployment → ⚙ Web app**.
2. **Execute as:** Me (your-email@gmail.com).
3. **Who has access:** Anyone.
4. Click **Deploy**. Google asks for OAuth consent the first time → **Allow**.

### 4. Find the URL that goes into the Parvaz app

Apps Script gives you a **Web app URL** that looks like:

```
https://script.google.com/macros/s/AKfycbyLONGRANDOMTOKEN/exec
                                    └────────┬────────┘
                                       deployment-id
                                  (the only piece you need)
```

You can re-open it any time: **Deploy → Manage deployments →** copy
the Web app URL.

**Sanity test:** open that URL in a browser. You should see
`{"e":"unauthorized"}` — that proves the deployment is live and
correctly rejecting unauthenticated calls.

### 5. Build the parvaz:// link

Combine the **deployment-id** from step 4 with the **AUTH\_KEY** from
step 2:

```
parvaz://<deployment-id>/<AUTH_KEY>#<display-name>
```

Concrete example (based on the values above):

```
parvaz://AKfycbyLONGRANDOMTOKEN/7dF9KmY3pQ8xV2nR5tL1aB4cE6gH9jM=#my-relay
```

The `#display-name` fragment is just a label the user will see in the
Parvaz UI — it never leaves the device.

### 6. Share the link via a SECURE messenger ONLY

This URL contains the AUTH\_KEY in plaintext. Anyone who sees it can
use your relay (and burn your quota or read traffic if you also added
a logger).

| Channel | OK? | Why |
|---|:-:|---|
| Signal | ✅ | end-to-end · default |
| Telegram **Secret Chat** | ✅ | end-to-end if you start a Secret Chat |
| Telegram regular chat | ⚠ | server-readable; better than WhatsApp, worse than Signal |
| WhatsApp | ❌ | Meta-readable cloud backup |
| SMS | ❌ | carrier-readable |
| Email | ❌ | server-readable, indexable |
| QR code shown in person | ✅ | nothing transits the network |

### 7. Limits and rotation

- **One relay per Google account** (Apps Script TOS — do not centralize).
- **~20k UrlFetch / day** free-tier quota.
- **30 s** per fetch · **6 min** per script execution.
- To rotate the key: change `AUTH_KEY` in `Code.gs` → **Deploy → Manage
  deployments → Edit (pencil)** → Version: New → Deploy. The
  deployment-id stays the same; only the AUTH\_KEY (and therefore the
  parvaz:// URL you share) changes.

References: [Apps Script · Web Apps](https://developers.google.com/apps-script/guides/web) ·
[`reference/apps_script/Code.gs`](./reference/apps_script/Code.gs).

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
