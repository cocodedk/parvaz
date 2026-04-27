# Settings Tab + In-App Update — Milestones

Detail companion to [`../settings-tab-update.md`](../settings-tab-update.md).

## TDD order

Each milestone strictly red→green→refactor. Failing test names are
the contract.

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

Pure refactor. New flag `onboardingComplete` passed in; ResetSection
composable takes it as a param and renders nothing when false.

- `SettingsSheetTest_HidesResetWhileOnboardingIncomplete`
- `SettingsSheetTest_ShowsResetAfterOnboardingComplete`

### M-settings-4 — UrlEditSection (inline edit, no CA reinstall)

- `UrlEditSectionTest_PrefillsFromCurrentAccess`
- `UrlEditSectionTest_RejectsInvalidUrl_ShowsFarsiError`
- `UrlEditSectionTest_AcceptsValidUrl_InvokesOnSave`
- `MainActivityUrlEditFlowTest` (instrumentation): paste new URL →
  save → assert ParvazSettings.load() returns new Access **and** CA
  file at `filesDir/parvaz-data/ca/ca.crt` is byte-identical to
  pre-edit.

CA-not-touched assertion is the load-bearing one.

### M-update-1 — Version compare

- `VersionTest_ParsesSemverWithVPrefix`
- `VersionTest_ParsesSemverWithoutVPrefix`
- `VersionTest_RejectsNonSemver`
- `VersionTest_LessThanGreaterThan`

### M-update-2 — GitHub releases client

- Real `releases/latest` JSON saved to `app/src/test/resources/`.
- Tests cover Success, NoAsset, Malformed, zero-sized asset paths.

### M-update-3 — Downloader + SHA-256 verifier

- `ApkDownloaderTest_StreamsToFile`
- `ApkDownloaderTest_VerifiesMatchingSha256`
- `ApkDownloaderTest_RejectsMismatchedSha256_DeletesPartial`
- `ApkDownloaderTest_EmitsProgress`
- `ApkDownloaderTest_NetworkErrorDeletesPartial`

### M-update-4 — Installer

Instrumentation only (PackageInstaller is opaque on JVM).

### M-update-5 — UI section + state machine

- `UpdateSectionTest_IdleToCheckingToAvailable`
- `UpdateSectionTest_ShowsUpToDate`
- `UpdateSectionTest_ShowsErrorOnNetworkFailure`
- `UpdateSectionTest_DisconnectsVpnBeforeDownload`

### M-update-6 — End-to-end on device

Manual checkpoint:
1. Build APK locally with version 0.1.0 so the live v0.1.5 reads as
   "newer".
2. Install on device, open settings, "check for updates", verify
   the changelog renders.
3. Tap "Install" → VPN disconnects → APK downloads → SHA verifies →
   system installer takes over.
4. Confirm the app post-install reports the new version.

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
| `update/UpdateController.kt` | 160 |
| `ui/main/AppRoot.kt` | 130 |
| MainActivity | ≤170 (after AppRoot extract) |

## Strings — additions to `res/values*/strings.xml`

FA-default; EN override mirror. Keys:

```
settings_gear_cd                  "تنظیمات"
settings_url_section_label        "نشانی پرواز"
settings_url_input_hint           "...//:parvaz بچسبانید"
settings_url_save_cta             "ذخیره"

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

## Testing checkpoints (where I'll ping)

1. After M-settings-2: gear icon visible on every screen — screenshot.
2. After M-settings-4: paste-and-save URL flow on device.
3. After M-update-5: settings → "check for updates" → "available"
   render — screenshot.
4. After M-update-6: full live install loop. **You drive this one** —
   I'll prep the APK and ping; you tap "Install" on the device.

## Risks & mitigations

| Risk | Mitigation |
|---|---|
| `REQUEST_INSTALL_PACKAGES` triggers Play Store policy concerns. | Already out-of-Play-Store (F-Droid + sideload only). No issue. |
| Download over the tunnel itself fails because github.com isn't on the SNI-rewrite list. | Force-disconnect VPN before download; bind to non-VPN network. |
| User has Settings → Install Unknown Apps disabled for Parvaz. | Deep-link to `ACTION_MANAGE_UNKNOWN_APP_SOURCES` with our package URI before first install. |
| User is mid-onboarding when they tap "Reset everything". | Hide the Reset section until onboarding completes. |
| Aggressive Iranian ISPs block raw `api.github.com`. | Out of scope — surface the network error clearly; don't retry forever. |

## Branch + commit plan

- One PR, branch `feat/settings-tab-update`, merged to main.
- Commits per milestone (`feat(settings):` and `feat(update):`
  prefixes; `fix(review):` for CodeRabbit follow-ups).
