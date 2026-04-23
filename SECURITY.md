# Security Policy

## Reporting a Vulnerability

Do **not** open a public GitHub issue for security vulnerabilities in Parvaz.

To report a vulnerability:
- Use the **"Report a vulnerability"** button on the Security tab (GitHub private advisory)
- Or email: babak@cocode.dk

We will acknowledge within 5 business days and aim to release a fix within
30 days of confirmation.

## Scope

In scope:
- **Go sidecar** (`core/`) — fronter dialer, MITM interceptor, Apps Script envelope relay, SOCKS5 listener, dispatcher, parvazd main
- **Android app** (`app/`) — VpnService subclass, tun2socks wrapper, sidecar launcher, MITM CA install flow, UI, settings storage (EncryptedSharedPreferences)
- **Build and release pipelines** — GitHub Actions workflows, signing, APK integrity
- **Git hooks and release artifacts** — pre-commit, commit-msg, keystore handling
- **Access URL format** — `parvaz://` scheme, QR encoding, clipboard handoff

Out of scope:
- The upstream Apps Script server (`reference/apps_script/Code.gs`) — report upstream
- User-deployed Google Apps Script deployments — these are out of our control
- Google Play Services / Android OS — report to Google
- MasterHttpRelayVPN-RUST — report to that project

## Threat Model Notes

Parvaz is a **circumvention aid**, not an anonymity system.

- **On-device MITM**: the app generates a CA on first launch and asks
  the user to install it in Android's user-CA store. The private key
  stays in app-private storage (`/data/data/dk.cocode.parvaz/ca/ca.key`).
  **Anyone who gains code execution as the Parvaz app** — root, debuggable
  build, ADB access with shell — can use the key to forge certs for any
  site the user's browser trusts via that CA. Uninstalling Parvaz deletes
  the key; removing the user CA from Android Settings revokes future trust.
- **The access key** is a shared secret between the app and `Code.gs`;
  stored in `EncryptedSharedPreferences` (Android Keystore-backed).
- **Google sees plaintext HTTP inside the relay** — `UrlFetchApp.fetch`
  reads the request in Apps Script. A compromised deployment exposes all
  tunneled traffic.
- **VpnService captures all phone traffic** while connected. Non-browser
  apps' handshakes fail (by design — see README scope) but the packets
  still route through Parvaz to the point of TLS termination.
- **Domain fronting resists DPI, not endpoint correlation.** An observer
  sees `www.google.com` traffic; volume/timing still correlate with user
  behaviour.
- **Access URL distribution is the weakest link.** If the Telegram
  message containing `parvaz://` is observed, the relay is compromised.
  Rotate `AUTH_KEY` periodically; redeploy `Code.gs`; distribute the new URL.

Do not use Parvaz as an attribution-resistance tool. It is for bypassing
network-layer blocks for low-sensitivity communications.

## Supported Versions

Only the latest release receives security fixes.
