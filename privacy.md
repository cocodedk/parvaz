# Privacy Policy — Parvaz (پرواز)

**App:** Parvaz (`dk.cocode.parvaz`)
**Developer:** CoCode.dk — Babak Bandpey
**Last updated:** 14 July 2026

> The canonical, always-current version of this policy is published at
> **https://cocodedk.github.io/parvaz/privacy.html** (bilingual English + فارسی).

> **Status:** Parvaz is pre-alpha software under active development. This policy describes the data
> flows built into the source code as published; it will be updated as the app evolves.

Parvaz is a Farsi-first **browser tunnel** for Android, built to help people reach the open web from
networks that filter it. It is **not a "no data" app**. To do its job it deliberately routes your
browsing through a **relay** — a Google Apps Script deployment that you, or a technical helper you
trust, set up and control.

> **The most important thing to understand:** everything you browse through Parvaz — the addresses
> you visit, the cookies and login tokens in your requests, and anything you type into web forms
> (including passwords) — passes through your relay operator and through Google in a form **they can
> read in plaintext**. This is a fundamental property of how Parvaz works, not a bug and not
> something a future update can remove. **Only use Parvaz with a relay you trust 100% — and never
> for accounts you cannot afford to lose.**

## 1. Who is responsible

Parvaz is developed and published by **CoCode.dk** (developer: Babak Bandpey). We write and release
the app; we do **not** operate any relay for you and we receive **no data** from your use of the
tunnel. The relay that carries your traffic is operated by whoever deployed it — often you yourself,
or a helper who shared a `parvaz://` link with you.

## 2. How your browsing traffic is handled (the relay)

When you tap Connect, Parvaz starts a local VPN on your device and launches a bundled network
component that intercepts your browser's HTTPS traffic on-device (a local "man-in-the-middle",
enabled by the certificate you install during setup). For each web request, it wraps the request into
a small JSON message and sends it to your relay at
`https://script.google.com/macros/s/<deployment-id>/exec`. To make the connection harder to block, it
is dialed against Google's servers using the front hostname `www.google.com` while actually
addressing `script.google.com`.

**What is sent to the relay (Google Apps Script)** — for every request you make in the browser while
connected:

- **Access key** — the secret from your `parvaz://` link, so the relay accepts the request.
- **Target URL** — the full address you are visiting.
- **HTTP method** — GET, POST, and so on.
- **Request headers** — **including `Cookie` and `Authorization` headers**, i.e. your session and
  login state for the sites you visit.
- **Request body** — **including form fields such as passwords**, uploads, and anything else you
  submit.

**What comes back** — the relay performs the request to the target site on your behalf and returns
the response status, the response headers (**including `Set-Cookie`**), and the response body. DNS
name resolution is performed as **DNS-over-HTTPS via Google Public DNS** (`dns.google`), carried over
the same relay/Google path rather than your local network's resolver.

> **Who can see this:** the operator of your relay can log every field above, in plaintext, for every
> page you load. **Google**, which runs Apps Script, Google's edge network, and Google Public DNS, is
> also technically able to observe this traffic and is governed by
> [Google's Privacy Policy](https://policies.google.com/privacy). If *you* deployed the relay to your
> own Google account, the only third party in the path is Google. If *someone else* operates it, they
> see your traffic too.

## 3. What Parvaz itself collects

- **No analytics, no crash reporting, no advertising, no telemetry.** The app contains no third-party
  analytics, ads, or tracking SDKs.
- **No account, no sign-up, no advertising identifier, no device fingerprinting.**
- CoCode.dk operates **no server** that receives your usage. We cannot see who uses Parvaz or what
  they browse.
- The display name in your `parvaz://` link is only a label shown in the app; it never leaves your
  device.

## 4. Data stored on your device

- **Relay access key** — stored in encrypted storage (`EncryptedSharedPreferences`) protected by a
  key held in the Android Keystore (hardware-backed where the device supports it).
- **Relay deployment ID, display name, and UI language** — stored in ordinary app preferences (not
  secret on their own).
- **The tunnel's private certificate authority (CA) key and certificate**, plus local tunnel state,
  in the app's private files. This private key never leaves the device.
- **A copy of the public CA certificate** is written to your Downloads folder during setup so you can
  install it into Android's trust store. You may delete this file after installing it.
- **A downloaded update file** (see below) may be kept temporarily in the app's cache.

The access key and the private CA material are **excluded from Android cloud backup and
device-to-device transfer**, so they do not leave the device through Google backup. Clearing the
app's data or uninstalling Parvaz removes this local data (you would then re-import your `parvaz://`
link and re-install the certificate).

## 5. Software updates and installing packages

- **Update check:** an anonymous request to
  `api.github.com/repos/cocodedk/parvaz/releases/latest` (no login, no query parameters, User-Agent
  `parvaz-app`). GitHub, as the host, necessarily sees the usual request metadata such as your IP
  address and the time of the request.
- **Download:** if a newer version exists and you choose to update, Parvaz downloads `Parvaz.apk` and
  its checksum from GitHub and verifies the SHA-256 before installing. The update download is
  deliberately performed **outside** the tunnel (the VPN is torn down first), so it uses your normal
  network connection to GitHub.
- **Install:** Parvaz hands the verified file to Android's system installer. The
  `REQUEST_INSTALL_PACKAGES` permission exists only for this. Android always shows its own
  confirmation dialog, and if you have not allowed installing unknown apps, Parvaz sends you to the
  relevant system settings first. **Parvaz never installs anything silently, and it only ever
  installs Parvaz itself** — never arbitrary third-party apps.

GitHub is a third party; its handling of request metadata is governed by
[GitHub's Privacy Statement](https://docs.github.com/site-policy/privacy-policies/github-general-privacy-statement).

## 6. Permissions and why they are used

- `INTERNET` — send tunnelled traffic to the relay and fetch updates.
- `ACCESS_NETWORK_STATE` — detect whether you are online before connecting.
- `FOREGROUND_SERVICE` and `FOREGROUND_SERVICE_SPECIAL_USE` — keep the VPN tunnel running as a
  foreground service while connected.
- `POST_NOTIFICATIONS` — show the ongoing notification that Android requires while the tunnel is
  active. Parvaz does not use notifications for marketing.
- `REQUEST_INSTALL_PACKAGES` — install a Parvaz update you have chosen (see section 5).

Establishing the VPN also triggers Android's own system consent dialog the first time. Parvaz does
not request location, contacts, camera, microphone, or media access.

## 7. Third parties

- **Your relay operator** — sees all traffic you route through the tunnel (see section 2). This may
  be you or a helper.
- **Google** — runs the Apps Script relay platform, the edge network the tunnel connects through, and
  the Google Public DNS resolver used for name lookups. See
  [Google's Privacy Policy](https://policies.google.com/privacy).
- **GitHub** — hosts release downloads and the update-check endpoint. See
  [GitHub's Privacy Statement](https://docs.github.com/site-policy/privacy-policies/github-general-privacy-statement).
- The app links to the developer's site ([cocode.dk](https://cocode.dk)) and LinkedIn; opening those
  uses your browser and is governed by their own policies.

## 8. Legal, risk, and trust

Parvaz is provided for personal, research, and educational use. Because of the
man-in-the-middle-via-relay design, the relay operator can read your traffic; deploy and use a relay
only on a Google account you control, and treat any shared relay as fully able to observe what you do.
Google's Apps Script terms restrict this kind of use — see the project documentation. Nothing here is
legal advice.

## 9. Children

Parvaz is not directed at children and does not knowingly collect data from anyone, including
children.

## 10. Changes

If this policy changes, the updated version will be posted here and on the website with a new
"last updated" date.

## 11. Contact

Questions about this policy can be sent to **bb@cocode.dk** (CoCode.dk, developer: Babak Bandpey).
