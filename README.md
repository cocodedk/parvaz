<p align="center">
  <img src="website/parvaz-mark.svg" alt="Parvaz" width="160"/>
</p>

<p align="center"><sub>— Welcome aboard · <strong>cocode.dk Airways</strong> —</sub></p>

# Parvaz (پرواز)

*Persian for "flight": a flight over the filter.*

Parvaz is an Android app that helps you open websites your network blocks. It sends your
browser's traffic through a **relay**, a small Google Apps Script program on a Google account.
A technical helper you trust sets up the relay and sends you an access link through
**Signal or a Telegram Secret Chat** (not WhatsApp). You install Parvaz, tap or paste the
`parvaz://` link, install a certificate in Android Settings (once), tap the **PARVAZ** button
(پرواز in Persian) and browse in **Chrome**. Use Chrome, Brave, Edge or Vivaldi; other
Chromium-based browsers vary, and DuckDuckGo does not work. Chrome trusts the certificate
Parvaz asks you to install, with no flags, no about:config and no root. Apps that are not
browsers (Instagram, the Telegram app, banking, streaming) do not accept that certificate and
do not work through Parvaz. See "Honest scope" below.

**Parvaz needs Android 11 or later and a 64-bit ARM phone.** It installs on Android 7 to 10,
but it cannot connect there. It is Persian by default, with English and Danish in its settings.

Parvaz follows the design of
[MasterHttpRelayVPN-RUST](https://github.com/therealaleph/MasterHttpRelayVPN-RUST). It adds an
app that is Persian by default, with guided steps for importing the access link and installing
the certificate.

## Website

- [English](https://parvaz.cocode.dk/)
- [فارسی (Persian)](https://parvaz.cocode.dk/fa/)
- [Dansk (Danish)](https://parvaz.cocode.dk/da/)

## Status

**Pre-alpha.** The app is built and runs, with the compatibility and speed limits described
below. See [`PLAN.md`](./PLAN.md) for the milestone list and
[`ARCHITECTURE.md`](./ARCHITECTURE.md) for the full data path.

## 🚨 Trust warning — read this first

**The relay operator can read every cookie, password and message you
send through the tunnel, in plain text.**

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

If the operator is **you** (you deployed your own `Code.gs`), you do not have to trust
another person. Google still processes your requests as plain text, and anyone who takes over
your phone or your Google account can see your traffic. If the operator is **someone else**,
every credential you submit is theirs.

## Features

- **Domain fronting** — TLS SNI `www.google.com`, HTTP Host `script.google.com`: DPI sees
  the front, Google's edge routes by Host.
- **Farsi-first onboarding** — paste or tap a `parvaz://` link, install the certificate
  once, tap the PARVAZ button (پرواز in Persian). Persian is the default language; English
  and Danish are in the settings.
- **Chrome out of the box** — Chrome trusts the user-installed Parvaz CA on Android with no
  flags. Brave, Edge and Vivaldi should work the same way. Native apps still reject user CAs
  by Android default.
- **Google sites use the relay too** — in the current Android version Google-owned sites go
  through the relay like every other site and use its quota (see "Honest scope").
- **Open source** — Kotlin + Compose UI, an embedded Go SOCKS5 sidecar, MIT licence.
- **No analytics** — no telemetry, no crash reporting, no ads; the access key is kept in
  `EncryptedSharedPreferences`.

## Honest scope

| | |
|---|---|
| ✅ **Chrome** (and Brave, Edge, Vivaldi) | **Recommended.** Trusts the user-installed CA with no setup; Chrome's Certificate Transparency enforcement has an explicit carve-out for chains rooted in a user CA, so the MITM is transparent. |
| ⚠️ Google-owned sites | In the current Android version these use the relay too and count against its quota. The code has a shortcut by site name, but Android hands Parvaz IP addresses, so the shortcut does not match (see [`docs/tls-flow.md`](./docs/tls-flow.md)). |
| ⚠️ Firefox | Not tested with Parvaz. Firefox keeps its own certificate list. It has a "Use third party CA certificates" option in Secret Settings ([Mozilla bug 1894053](https://bugzilla.mozilla.org/show_bug.cgi?id=1894053)), which Parvaz has not been checked with. The old Nightly issue [fenix#18990](https://github.com/mozilla-mobile/fenix/issues/18990) is closed. **Use Chrome instead.** |
| ❌ DuckDuckGo browser | Confirmed broken. Chromium-based but configured more strictly, so it rejects the Parvaz CA |
| ❌ Android 10 and older | The app installs (its minimum is Android 7) but cannot connect: it passes the tunnel to its helper program with a call that needs Android 11 |
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

Tap **Open Settings** in Parvaz. Parvaz saves a certificate file and opens Android Settings
as close as it can to the install screen. The file is named `parvaz-ca-` plus a 12-character
code and `.crt`. If you cannot find it, tap **Show file** in Parvaz. From there:

- **Samsung (One UI 6 / Galaxy):** Security and privacy → More security
  settings → Install from device storage → CA certificate → choose the
  newest file whose name starts with `parvaz-ca-` and ends with `.crt` →
  tap **Install anyway**.
- **Pixel / stock Android (14, 15):** Security & privacy → More
  security & privacy → Encryption & credentials → Install a
  certificate → CA certificate → choose the newest `parvaz-ca-….crt`
  file → **Install anyway**.

You'll be asked for your screen-lock PIN or fingerprint. Walk back to
Parvaz (gesture back); it auto-detects the cert and advances.

> Pre-condition: your phone must have a screen lock (PIN, pattern, or
> password) — Android refuses CA install otherwise.

### 2. Tap PARVAZ, then open Chrome

Parvaz first asks you to allow Android's VPN connection. Tap **Allow**. Then tap the
**PARVAZ** button (**پرواز** in Persian) on Parvaz's main screen. When it changes to
**IN FLIGHT** (**در پرواز**), open **Chrome** and browse as usual. HTTPS pages go through
Parvaz with no further setup. Tap the button again to disconnect. Brave, Edge and Vivaldi
should work the same way; the Parvaz CA you just installed is the only piece they need.

> Throughput is currently around 5 KB/s and WebSockets do not work. These are limits
> of the underlying Apps Script relay, not the browser. Plain HTTPS pages load slowly.

## Download

<!-- cocode-apps:install:start -->
- Coming to F-Droid
- [Download the Android installation file (APK) from GitHub](https://github.com/cocodedk/parvaz/releases/latest/download/Parvaz.apk)
- [Add the app to Obtainium, an app that keeps it up to date](https://apps.obtainium.imranr.dev/redirect?r=obtainium://add/https://github.com/cocodedk/parvaz)
<!-- cocode-apps:install:end -->

[SHA-256 of the latest APK](https://github.com/cocodedk/parvaz/releases/latest/download/Parvaz.apk.sha256)
· [All releases](https://github.com/cocodedk/parvaz/releases)

CLI install + integrity check (recommended on hostile networks):

```sh
curl -LO https://github.com/cocodedk/parvaz/releases/latest/download/Parvaz.apk
curl -LO https://github.com/cocodedk/parvaz/releases/latest/download/Parvaz.apk.sha256
sha256sum -c Parvaz.apk.sha256        # must print: Parvaz.apk: OK
adb install Parvaz.apk                 # or sideload through your file manager
```

Install from GitHub for now (sideload); F-Droid is planned. Google Play is not a viable channel.

## Helper guide — deploy the relay on Apps Script

You (the helper) need **a free Google account**. The end user never
opens script.google.com. Total time: ~5 minutes, once. Free quota: 20,000 URL
fetches per day for a consumer Google account, shared by all of that account's scripts.

### 1. Create the Apps Script project

1. Open <https://script.google.com> → click **New project**.
2. Delete the default `Code.gs` contents.
3. Open
   [`apps_script/Code.gs`](./apps_script/Code.gs)
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

**Sanity test:** open that URL in a browser. You should see a welcome page ("Welcome. This
application is running normally."). That shows the deployment is live. It does not test the
AUTH\_KEY; only a POST request with the key does that.

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

The `#display-name` fragment is just a label for the link. Parvaz stores it but does not show it
anywhere in the app. It is not sent to the relay, but it is stored in the app's ordinary
settings, which Android may copy in an app backup or a phone-to-phone transfer.

### 6. Share the link via a SECURE messenger ONLY

This URL contains the AUTH\_KEY in plaintext. Anyone who sees it can
use your relay (and burn your quota or read traffic if you also added
a logger).

| Channel | OK? | Why |
|---|:-:|---|
| Signal | ✅ | end-to-end · default |
| Telegram **Secret Chat** | ✅ | end-to-end encrypted, only if you start a Secret Chat |
| Telegram regular chat | ❌ | not end-to-end encrypted; Telegram's servers can read it |
| WhatsApp | ❌ | messages are end-to-end encrypted, but a chat backup can be readable unless encrypted backups are on; avoid |
| SMS | ❌ | carrier-readable |
| Email | ❌ | server-readable, indexable |
| QR code shown in person | ❌ | nothing transits the network, but Parvaz cannot scan QR codes yet |

### 7. Limits and rotation

- Use a Google account you control, follow Google's terms and quotas, and do not centralize
  one relay for many people.
- **20,000 URL fetches a day** on a consumer account, shared by all its scripts.
- Parvaz waits up to **30 s** for the relay's reply headers. Apps Script limits one script
  run to **6 min**.
- To rotate the key: change `AUTH_KEY` in `Code.gs` → **Deploy → Manage
  deployments → Edit (pencil)** → Version: New → Deploy. The
  deployment-id stays the same; only the AUTH\_KEY (and therefore the
  parvaz:// URL you share) changes.

References: [Apps Script · Web Apps](https://developers.google.com/apps-script/guides/web) ·
[`apps_script/Code.gs`](./apps_script/Code.gs).

## Privacy

- Parvaz has no analytics, no telemetry, no crash reporting and no ads. No servers are
  operated by us, and there are no accounts and no sign-in.
- Your browser traffic goes through the Google Apps Script relay your helper deployed (or
  you did). In the current Android version this includes Google-owned sites. **The relay
  operator can read what goes through the relay in plain text**; see the trust warning
  above.
- The access key is kept in `EncryptedSharedPreferences`. The certificate's private key is
  generated on your phone and never leaves the app's private storage.
- Permissions: network access (`INTERNET`, `ACCESS_NETWORK_STATE`), the VPN service, a
  foreground service, and `POST_NOTIFICATIONS` (Android 13+) for the connected notification.
- The About screen has buttons that open web pages (the privacy policy, the website, the
  source code, the issues page and the latest release). Parvaz hands each address to your
  browser only when you tap the button. The full policy is at
  <https://parvaz.cocode.dk/privacy/>.

## Build

**Prerequisites:** Android Studio (latest), JDK 17, Go 1.24+.

Build the Go core for the phone before you assemble the APK. Without it the APK builds, but
the connection cannot start:

```sh
CGO_ENABLED=0 GOOS=android GOARCH=arm64 \
    go build -C core -o ../app/src/main/jniLibs/arm64-v8a/libparvaz.so ./cmd/parvazd
```

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
│   ├── mitm/      local TLS interception: CA, leaf certificates, TLS server
│   ├── dispatcher/ routing: direct, SNI rewrite or relay
│   └── cmd/parvazd/ sidecar main → libparvaz.so
├── reference/     Upstream MasterHttpRelayVPN Python — read-only
└── website/       GitHub Pages (English, Persian, Danish)
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

## Contributing

Local setup, git hooks, the build and test commands, coding style and the pull request
checklist are in [CONTRIBUTING.md](./CONTRIBUTING.md). Bugs and ideas go to the
[issues page](https://github.com/cocodedk/parvaz/issues).

## Alternative: use MasterHttpRelayVPN-RUST directly

If you do not need an app that is Persian by default, with guided setup,
[MasterHttpRelayVPN-RUST](https://github.com/therealaleph/MasterHttpRelayVPN-RUST)
offers prebuilt APKs. It has the same architecture and a full guide in English and Persian.

## Legal / ToS

Google Apps Script's terms may forbid this use. Deploy `Code.gs` to your
**own** Google account only. Personal, research, and educational use
only. See upstream disclaimer.

## Author

**Babak Bandpey** — [cocode.dk](https://cocode.dk) · [LinkedIn](https://linkedin.com/in/babakbandpey) · [GitHub](https://github.com/cocodedk)

## License

MIT | © 2026 [Cocode](https://cocode.dk) | Created by [Babak Bandpey](https://linkedin.com/in/babakbandpey)
