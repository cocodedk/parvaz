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
- **Go sidecar** (`core/`) — fronter dialer, WebSocket relay, SOCKS5 listener, `parvazd` main binary
- **Cloudflare Worker** (`worker/worker.js`) — auth handling, socket piping, error surfaces
- **Android app** (`app/`) — VpnService subclass, tun2socks wrapper, sidecar launcher, UI, settings storage (EncryptedSharedPreferences)
- **Build and release pipelines** — GitHub Actions workflows, signing, APK integrity
- **Git hooks and release artifacts** — pre-commit, commit-msg, keystore handling
- **Access URL format** — `parvaz://` scheme, QR encoding, clipboard handoff

Out of scope:
- Cloudflare's infrastructure / Workers runtime — report to Cloudflare
- Google Play Services / Android OS — report to Google
- MasterHttpRelayVPN upstream (`reference/`) — the product no longer depends on it

## Threat Model Notes

Parvaz is a **circumvention aid**, not an anonymity system.

- **VpnService captures all phone traffic.** Every outbound TCP flow from every app goes through Parvaz while connected. The app has full visibility into traffic at the TUN layer.
- **Cloudflare sees plaintext TCP bytes** past the Worker's WebSocket wrap — because the Worker opens a raw outbound socket to the destination. A compromised Worker deployment exposes all traffic. Cloudflare itself is also on the path.
- **The access key** is a shared secret between the app and `worker.js`; it is stored on-device in `EncryptedSharedPreferences` (Android Keystore-backed).
- **Domain fronting resists DPI, not endpoint correlation.** An observer sees Cloudflare traffic. They can still correlate volume and timing with user behaviour.
- **No anonymity guarantees.** Do not use Parvaz against adversaries who can compel Cloudflare, or who can observe traffic on both endpoints.
- **Access URL distribution channel is the weakest link.** If the Telegram message containing the `parvaz://` URL is observed, the relay is compromised. Rotate keys periodically.

Do not use Parvaz as a tool for attribution resistance. It is a tool for
bypassing network-layer blocks for low-sensitivity communications.

## Supported Versions

Only the latest release receives security fixes.
