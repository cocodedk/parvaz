# Settings Tab + In-App Update — Architecture

Detail companion to [`../settings-tab-update.md`](../settings-tab-update.md).

## UI: shared scaffold

Today every screen is rendered directly under MainActivity's
`Scaffold`. Refactor to a thin wrapper:

```
ui/settings/SettingsScaffold.kt   (≤120L)
    Box(fillMaxSize) { content(); IconButton at TopStart → onOpenSettings }
```

Every top-level screen (`SplashScreen`, `ImportAccessScreen`,
`CaInstallScreen`, `VpnPermissionScreen`, `ReadinessScreen`,
`MainScreen`) is wrapped in `SettingsScaffold`. The MainScreen
long-press gesture is preserved (parallel entry point, costs nothing).

The scaffold renders:
- A right-aligned `IconButton` (TopStart, so it never collides with
  the language toggle pinned at TopEnd inside `OnboardingHost`) with
  the gear icon → opens settings sheet.

`MainActivity` lifts `showSettingsSheet` state up so it works the
same across both onboarding and main. The actual routing lives in
`ui/main/AppRoot.kt` (extracted to keep MainActivity ≤200 lines).

## Settings sheet — module split

The sheet grows past 200 lines if we put everything in one file.
Split:

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

## Update logic — domain

```
update/Version.kt                semver parse + compare (pure, JVM-testable)
update/GitHubReleasesClient.kt   fetchLatest() → FetchResult
update/ApkDownloader.kt          download(url, sha256) → ApkDownloadOutcome
update/ApkInstaller.kt           hands File to ACTION_VIEW (PackageInstaller)
update/UpdateController.kt       AndroidViewModel composing the four
update/UpdateState.kt            sealed UpdateState for the UI
```

Networking: `HttpURLConnection` (stdlib — fine for two endpoints).
JSON parsing via `org.json` (built-in on Android; explicit
`testImplementation` for JVM unit tests).

## State machine — update screen

```
IDLE
  ├─ check() → CHECKING
CHECKING
  ├─ no newer  → UP_TO_DATE
  ├─ newer     → AVAILABLE(release)
  ├─ network   → ERROR(reason)
AVAILABLE(release)
  ├─ install() → DISCONNECTING_VPN
                   ↓
                 DOWNLOADING(progressBytes)
                   ↓ (sha256 verify)
                 INSTALLER_HANDOFF (system installer takes over)
                 or DOWNLOAD_ERROR / VERIFY_ERROR
                 or NEEDS_UNKNOWN_SOURCES(release)
NEEDS_UNKNOWN_SOURCES(release)
  ├─ install() resumes the download from this state
```

Persisted nowhere — this is in-flight UI state only. A re-open of the
sheet after process death starts at IDLE. The `release` is carried
through `NeedsUnknownSources` so the user can resume after flipping
the system permission without re-running `check()`.

## Permissions

Add to `AndroidManifest.xml`:
- `<uses-permission android:name="android.permission.REQUEST_INSTALL_PACKAGES"/>`

`INTERNET` is already declared. The runtime confirmation for "install
unknown apps" is a system intent (`ACTION_MANAGE_UNKNOWN_APP_SOURCES`)
that we deep-link to before the first install attempt if
`PackageManager.canRequestPackageInstalls()` returns false.

`FileProvider` (already declared) gets a new `cache-path` mapping
under `parvaz-update/` so the downloaded APK can be handed to
`ACTION_VIEW` without `WRITE_EXTERNAL_STORAGE`.

## VPN coordination

`MainViewModel` already exposes `disconnect()` and a `ConnectionState`
flow. The update flow:

1. Read connection state.
2. If CONNECTED, `disconnect()` and wait up to 4s for `DISCONNECTED`.
3. Bind the process to a non-VPN network via
   `ConnectivityManager.bindProcessToNetwork(non-vpn-network)` so the
   request bypasses any auto-reconnect.
4. The install coroutine wraps its body in a `try/finally` that
   releases the binding (`bindProcessToNetwork(null)`) on every
   non-`Launched` terminal state and on cancellation. Otherwise a
   user re-enabling Parvaz after a failed update would silently
   bypass the tunnel until process death.
5. On cancel / SHA mismatch / 404, surface the error and offer
   "reconnect VPN" CTA.

If the user wants to manually reconnect after a successful install,
that's their call — the new APK takes over once the system installer
finishes.
