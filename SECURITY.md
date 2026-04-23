# Security Policy

## Reporting a Vulnerability

Do **not** open a public GitHub issue for security vulnerabilities in Parvaz.

To report a vulnerability:
- Use the **"Report a vulnerability"** button on the Security tab of this repository (GitHub private advisory)
- Or email: babak@cocode.dk

We will acknowledge within 5 business days and aim to release a fix within
30 days of confirmation.

## Scope

In scope:
- **Go core** (`core/`) — protocol envelope, fronting dialer, codec, relay, SOCKS5 listener, sidecar binary
- **Android app** (`app/`) — VpnService subclass, tun2socks wrapper, sidecar launcher, settings storage, UI
- **Build and release pipelines** — GitHub Actions workflows, signing, APK integrity
- **Git hooks and release artifacts** — pre-commit, commit-msg, keystore handling

Out of scope:
- The upstream Apps Script server (`reference/apps_script/Code.gs`) — report upstream
- User-deployed Google Apps Script deployments — these are out of our control
- Google Play Services / Android OS vulnerabilities — report to Google

## Threat Model Notes

Parvaz is a **circumvention aid**, not an anonymity system. It carries live
traffic through a user-deployed Apps Script relay using domain fronting.

- **VpnService captures all phone traffic.** Every outbound TCP flow from
  every app on the device goes through Parvaz while connected. The app has
  full visibility into unencrypted traffic at the TUN layer, and into the
  destinations of encrypted traffic.
- **Google sees plaintext HTTP inside the relay** (TLS terminates at
  Google's edge). A compromised Apps Script deployment exposes all traffic.
- **The `auth_key` is a shared secret** between Parvaz and the user's
  `Code.gs` — treat it like a password. It is stored on-device in
  `EncryptedSharedPreferences` (Android Keystore-backed).
- **Domain fronting resists DPI, not endpoint correlation.** A network
  observer sees `www.google.com` traffic; they can still correlate volume
  and timing with user behavior.
- **No anonymity guarantees.** Do not use Parvaz for threat models that
  assume an honest-but-curious intermediary (Google).

Do not use Parvaz against adversaries who can compel Google, or who can
observe traffic on both endpoints.

## Supported Versions

Only the latest release receives security fixes.
