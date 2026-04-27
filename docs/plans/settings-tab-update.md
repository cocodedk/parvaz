# Settings Tab + In-App Update — Plan

**Branch:** `feat/settings-tab-update`
**Owner:** bb · **Drafted:** 2026-04-27 · **Status:** awaiting user approval

## Goal

Two related features shipped together so the new global Settings surface
has something useful in it from day one:

1. A **gear icon visible on every screen** that opens a global Settings
   sheet. Visible in onboarding *and* on Main.
2. The sheet exposes:
   - **Edit Parvaz URL** — paste a new `parvaz://...` and save in place.
     No CA reinstall (the CA is device-local; changing the upstream
     Apps Script deployment doesn't invalidate the existing root).
   - **Language toggle** (existing — moves over from `MainSettingsSheet`).
   - **Reset everything** (existing — destructive, dialog-gated, wipes
     access + onboarding flag and routes back through onboarding).
   - **Check for updates** (new) — hits GitHub Releases API, compares
     to `BuildConfig.VERSION_NAME`, shows changelog if newer.
   - **Install update** (new) — downloads `Parvaz.apk`, verifies
     against `Parvaz.apk.sha256`, hands to system `PackageInstaller`.
     Auto-disconnects the VPN first so the download bypasses the
     tunnel (otherwise github.com goes through Apps Script and the
     tunnel's MITM truststore breaks the TLS handshake).

## Decisions captured (from chat)

- "URL" = the **Parvaz access URL** (parvaz://deployment-id/key). Not
  the GitHub release URL.
- Settings entry must be **visible from everywhere**, including
  onboarding screens. (User overrode my "main + splash only" suggestion.)
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

## Architectural shape

### UI: shared scaffold

Today every screen is rendered directly under MainActivity's
`Scaffold`. We refactor to a thin wrapper:

```
ui/settings/SettingsScaffold.kt   (≤120L)
    Scaffold(topBar = SettingsTopBar(onOpenSettings))
        content()
```

Every top-level screen (`SplashScreen`, `ImportAccessScreen`,
`CaInstallScreen`, `VpnPermissionScreen`, `ReadinessScreen`,
`MainScreen`) is wrapped in `SettingsScaffold`. The MainScreen long-press
gesture is preserved (parallel entry point, costs nothing).

The TopBar shows:
- Brand wordmark "پرواز" (Vazirmatn, small, leading).
- A right-aligned `IconButton` with the gear icon → opens settings sheet.

`MainActivity` lifts `showSettingsSheet` state up so it works the same
across both onboarding and main.

### Settings sheet — module split

The sheet grows past 200 lines if we put everything in one file. Split:

```
ui/settings/SettingsSheet.kt          orchestrator (≤150L)
ui/settings/UrlEditSection.kt         inline parvaz:// edit (≤170L)
ui/settings/LanguageSection.kt        moved from MainSettingsSheet
ui/settings/ResetSection.kt           moved from MainSettingsSheet
ui/settings/UpdateSection.kt          check + install UI (≤180L)
```

Each section is a standalone `@Composable`. The orchestrator stitches
them in a `Column` inside `ModalBottomSheet`. All caller hoisting via
lambdas — no ViewModel inside the sheet, state lifted to the Activity
or to a per-section ViewModel where async I/O is involved.

### Update logic — domain

```
update/Version.kt                semver parse + compare (pure, JVM-testable)
update/GitHubReleasesClient.kt   suspend fetchLatest() → ReleaseInfo
update/ApkDownloader.kt          suspend download(url, sha256) → File
update/ApkInstaller.kt           hands File to PackageInstaller session
update/UpdateRepository.kt       composes the four into checkForUpdate() + install(release)
```

Networking: **OkHttp** if already pulled in, else `HttpURLConnection`
(stdlib — fine for two endpoints). Quick check needed during M-update-1.

### State machine — update screen

```
IDLE
  ├─ check() → CHECKING
CHECKING
  ├─ no newer  → UP_TO_DATE  (auto-resets to IDLE after dismiss)
  ├─ newer     → AVAILABLE(release)
  ├─ network   → ERROR(reason)
AVAILABLE(release)
  ├─ install() → DISCONNECTING_VPN
                   ↓
                 DOWNLOADING(progressBytes)
                   ↓ (sha256 verify)
                 INSTALLING (system installer takes over)
                 or DOWNLOAD_ERROR / VERIFY_ERROR
```

Persisted nowhere — this is in-flight UI state only. A re-open of the
sheet after process death starts at IDLE.

### Permissions

Add to `AndroidManifest.xml`:
- `<uses-permission android:name="android.permission.REQUEST_INSTALL_PACKAGES"/>`

`INTERNET` is already declared. The runtime confirmation for
"install unknown apps" is a system intent (`ACTION_MANAGE_UNKNOWN_APP_SOURCES`)
that we deep-link to before the first install attempt if
`PackageManager.canRequestPackageInstalls()` returns false.

### VPN coordination

`MainViewModel` already exposes `disconnect()` and a `ConnectionState`
flow. The update flow:

1. Read connection state.
2. If CONNECTED, `disconnect()` and wait for `DISCONNECTED`.
3. Bind the OkHttp/HttpURLConnection call to a non-VPN
   `ConnectivityManager.bindProcessToNetwork(non-vpn-network)` to
   guarantee the request bypasses any auto-reconnect.
4. On cancel / SHA mismatch / 404, surface error and offer
   "reconnect VPN" CTA.

If the user wants to manually reconnect after a successful install,
that's their call — the new APK takes over once the system installer
finishes.

## TDD order

Each milestone strictly red→green→refactor. Failing test names are the
contract.

### M-settings-1 — SettingsScaffold

JVM Compose tests under `app/src/test/`:

- `SettingsScaffoldTest_GearIconHasContentDescription`
- `SettingsScaffoldTest_GearIconClickInvokesCallback`
- `SettingsScaffoldTest_RendersChildContent`

Then wire one screen (Splash) to use it. Visual verify via screenshot
on connected device → ping user.

### M-settings-2 — Wire scaffold across all screens

- `SplashScreenTest_HasSettingsGear`
- `ImportAccessScreenTest_HasSettingsGear`
- `CaInstallScreenTest_HasSettingsGear`
- `VpnPermissionScreenTest_HasSettingsGear`
- `MainScreenTest_HasSettingsGear`
- `MainScreenTest_LongPressStillOpensSettings` (don't regress M13b)

### M-settings-3 — Move existing sheet sections out

Pure refactor. Existing `MainSettingsSheetTest` cases should pass
unchanged after the split.

- Delete `MainSettingsSheet.kt`; replace MainActivity reference with
  `SettingsSheet`.
- New flag `onboardingComplete` passed in; ResetSection composable
  takes it as a param and renders nothing when false.
- `SettingsSheetTest_HidesResetWhileOnboardingIncomplete`
- `SettingsSheetTest_ShowsResetAfterOnboardingComplete`

### M-settings-4 — UrlEditSection (inline edit, no CA reinstall)

- `UrlEditSectionTest_PrefillsFromCurrentAccess`
- `UrlEditSectionTest_RejectsInvalidUrl_ShowsFarsiError`
- `UrlEditSectionTest_AcceptsValidUrl_InvokesOnSave`
- `MainActivityUrlEditFlowTest` (instrumentation): paste new URL →
  save → assert ParvazSettings.load() returns new Access **and**
  CA file at `filesDir/parvaz-data/ca/ca.crt` is byte-identical to
  pre-edit.

CA-not-touched assertion is the load-bearing one. Reuses
`AccessImport.tryExtractFromUri` so the parser stays in one place.

### M-update-1 — Version compare

Pure Kotlin. `Version.parse("v0.1.4") -> Version(0,1,4)`,
`Version.compareTo`, edge cases (prefix `v`, missing patch, prerelease
suffix → ignore for now).

- `VersionTest_ParsesSemverWithVPrefix`
- `VersionTest_ParsesSemverWithoutVPrefix`
- `VersionTest_RejectsNonSemver`
- `VersionTest_LessThanGreaterThan`

### M-update-2 — GitHub releases client

- `GitHubReleasesClientTest_ParsesLatestPayload` (fixture: real
  `releases/latest` JSON saved to `app/src/test/resources/`)
- `GitHubReleasesClientTest_ExtractsApkAssetUrl`
- `GitHubReleasesClientTest_ExtractsSha256AssetUrl`
- `GitHubReleasesClientTest_HandlesMissingApkAsset`
- `GitHubReleasesClientTest_RetriesOnce_On5xx`

### M-update-3 — Downloader + SHA-256 verifier

JVM with `MockWebServer` (already in androidx.test):

- `ApkDownloaderTest_StreamsToFile`
- `ApkDownloaderTest_VerifiesMatchingSha256`
- `ApkDownloaderTest_RejectsMismatchedSha256_DeletesPartial`
- `ApkDownloaderTest_EmitsProgress`
- `ApkDownloaderTest_CancellationDeletesPartial`

### M-update-4 — Installer

- Instrumentation only (PackageInstaller is opaque on JVM).
  `ApkInstallerInstrumentedTest_OpensInstallerActivity` (assert
  intent extras + flags; do not actually install in CI).

### M-update-5 — UI section + state machine

- `UpdateSectionTest_IdleToCheckingToAvailable`
- `UpdateSectionTest_ShowsUpToDate`
- `UpdateSectionTest_ShowsErrorOnNetworkFailure`
- `UpdateSectionTest_DisconnectsVpnBeforeDownload`

### M-update-6 — End-to-end on device

Manual checkpoint. I'll:
1. Build APK locally with version bumped to `v0.1.999` (so the live
   v0.1.4 reads as "newer" relative to nothing — actually no, I'll
   leave the local build at 0.1.0 so v0.1.4 IS newer, and verify the
   real download/install loop against the live release).
2. Install on device, open settings, "check for updates", verify the
   v0.1.4 changelog renders.
3. Tap "Install" → VPN disconnects → APK downloads → SHA verifies →
   system installer takes over.
4. Confirm the app post-install has version `v0.1.4`.

This is where I'll ping you for testing.

## File budget

Targets (all ≤200L):

| File | est. lines |
|---|---|
| `ui/settings/SettingsScaffold.kt` | 100 |
| `ui/settings/SettingsSheet.kt` | 130 |
| `ui/settings/UrlEditSection.kt` | 160 |
| `ui/settings/LanguageSection.kt` | 60 |
| `ui/settings/ResetSection.kt` | 80 |
| `ui/settings/UpdateSection.kt` | 180 |
| `update/Version.kt` | 80 |
| `update/GitHubReleasesClient.kt` | 150 |
| `update/ApkDownloader.kt` | 170 |
| `update/ApkInstaller.kt` | 120 |
| `update/UpdateRepository.kt` | 100 |
| `update/UpdateViewModel.kt` | 160 |
| MainActivity diff | +30 |

## Strings — additions to `res/values*/strings.xml`

FA-default; EN override mirror. New keys:

```
settings_gear_cd                  "تنظیمات"
settings_url_section_label        "نشانی پرواز"
settings_url_input_hint           "...//:parvaz بچسبانید"
settings_url_save_cta             "ذخیره"
settings_url_invalid_error        (reuses existing AccessParseException msgs)

settings_update_section_label     "به‌روزرسانی"
settings_update_check_cta         "بررسی به‌روزرسانی"
settings_update_uptodate          "نسخهٔ شما به‌روز است"
settings_update_available         "نسخهٔ جدید: %1$s"
settings_update_install_cta       "نصب نسخهٔ جدید"
settings_update_disconnecting     "در حال قطع پرواز…"
settings_update_downloading       "در حال دانلود… %1$s٪"
settings_update_verifying         "در حال بررسی امضا"
settings_update_installer_handoff "تحویل به نصب‌کنندهٔ سیستم…"
settings_update_error_network     "اتصال به GitHub برقرار نشد"
settings_update_error_sha         "امضای فایل نامعتبر است"
settings_update_error_no_asset    "این انتشار فایل APK ندارد"
settings_update_release_notes     "تغییرات این نسخه"
```

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

## Testing checkpoints (where I'll ping)

1. After M-settings-2: gear icon visible on every screen — screenshot.
2. After M-settings-4: paste-and-save URL flow on device — short demo
   of "edit URL → connect with new deployment, no CA prompt".
3. After M-update-5: settings → "check for updates" → "available"
   render — screenshot.
4. After M-update-6: full live install loop against v0.1.4. **You drive
   this one** — I'll prep the APK and ping; you tap "Install" on the
   device.

## Risks & mitigations

| Risk | Mitigation |
|---|---|
| `REQUEST_INSTALL_PACKAGES` triggers Play Store policy concerns. | Already out-of-Play-Store (F-Droid + sideload only per PLAN.md). No issue. |
| Download over the tunnel itself fails because github.com isn't on the SNI-rewrite list and the relay's MITM mangles it. | Force-disconnect VPN before download; bind to non-VPN network. |
| User has Settings → Install Unknown Apps disabled for Parvaz. | Deep-link to `ACTION_MANAGE_UNKNOWN_APP_SOURCES` with our package URI before first install. |
| User is mid-onboarding when they tap "Reset everything". | Existing confirmation dialog stays; copy gets sharper to mention onboarding will restart. |
| Inline URL edit creates a stale Access mid-connection. | If VPN is connected when the user saves a new URL, prompt to reconnect. |
| Aggressive Iranian ISPs block raw `api.github.com`. | Out of scope — if GitHub is blocked the user can't update anyway. Surface the network error clearly; don't retry forever. |

## Branch + commit plan

- One PR, branch `feat/settings-tab-update`, merged to main.
- Commits per milestone (`feat(settings):` and `feat(update):` prefixes).
- Squash-merge if it grows beyond ~12 commits.
- Bump `version.txt` to `0.2.0` on the merge commit (settings is a
  user-visible feature; warrants a minor).

## Open questions parked for later

- **Translatable changelogs.** GitHub release bodies are in English.
  Render as-is for now; translate-on-the-fly is too invasive.
- **F-Droid metadata.** When we list on F-Droid, in-app updaters are
  banned — the section will need to be conditionally hidden. Add a
  build flavor `fdroid` later; not now.
