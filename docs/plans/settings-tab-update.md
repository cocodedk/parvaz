# Settings Tab + In-App Update — Plan

**Branch:** `feat/settings-tab-update` (merged in PR #52, commit `5402388`)
**Drafted:** 2026-04-27 · **Shipped:** 2026-04-27 · **Status:** done

## Goal

Two related features shipped together so the new global Settings
surface has something useful in it from day one:

1. A **gear icon visible on every screen** that opens a global
   Settings sheet. Visible in onboarding *and* on Main.
2. The sheet exposes:
   - **Edit Parvaz URL** — paste a new `parvaz://...` and save in
     place. No CA reinstall (the CA is device-local; changing the
     upstream Apps Script deployment doesn't invalidate the existing
     root).
   - **Language toggle** (existing — moved over from
     `MainSettingsSheet`).
   - **Reset everything** (existing — destructive, dialog-gated,
     wipes access + onboarding flag and routes back through
     onboarding). Hidden until onboarding completes per the
     "least distracting" rule.
   - **Check for updates** (new) — hits GitHub Releases API,
     compares to `BuildConfig.VERSION_NAME`, shows changelog if
     newer.
   - **Install update** (new) — downloads `Parvaz.apk`, verifies
     against `Parvaz.apk.sha256`, hands to system `PackageInstaller`.
     Auto-disconnects the VPN first so the download bypasses the
     tunnel.

## Decisions captured (from chat)

- "URL" = the **Parvaz access URL** (parvaz://deployment-id/key). Not
  the GitHub release URL.
- Settings entry must be **visible from everywhere**, including
  onboarding screens.
- URL edit must **not** trigger CA reinstall.
- Update check is **manual only** — no auto-check on launch
  (consistent with the project's zero-telemetry principle).
- Update download **auto-disconnects the VPN** during the fetch,
  reconnects on cancel/failure (caller's choice on success — typically
  the device reboots through the installer anyway).
- **Onboarding mode is "least distracting":** when onboarding is
  incomplete, the gear stays visible on every screen but the sheet
  hides the **Reset everything** button (URL edit, language, and
  update check are still available). Only after onboarding completes
  does the destructive wipe surface.

## Detail files

- [`settings-tab-update/architecture.md`](settings-tab-update/architecture.md) —
  UI scaffold, sheet module split, update domain layout, state
  machine, permissions, VPN coordination.
- [`settings-tab-update/milestones.md`](settings-tab-update/milestones.md) —
  TDD order, file budget, string keys, testing checkpoints, branch
  plan, risks.

## Out of scope (explicit)

- **Auto-update / background sync** — manual only.
- **Delta updates** — full APK each time.
- **Update channel selection** (stable / nightly) — only the latest
  non-prerelease release.
- **Signature pinning beyond SHA-256** — Android's PackageInstaller
  already verifies the APK signing key matches the installed app's
  signing key; we don't reimplement that.
- **Rollback** — Android side handles it; if the install fails after
  PackageInstaller takes over, the existing app remains.
- **GitHub API auth** — anonymous (60 req/hr/IP). Plenty for manual
  use. If we ever need higher, switch to ETags first.

## Open questions parked for later

- **Translatable changelogs.** GitHub release bodies are in English.
  Render as-is for now; translate-on-the-fly is too invasive.
- **F-Droid metadata.** When we list on F-Droid, in-app updaters are
  banned — the section will need to be conditionally hidden. Add a
  build flavor `fdroid` later; not now.
