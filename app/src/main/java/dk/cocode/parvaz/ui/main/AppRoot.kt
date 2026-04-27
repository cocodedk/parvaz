package dk.cocode.parvaz.ui.main

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Scaffold
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.testTagsAsResourceId
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import dk.cocode.parvaz.BuildConfig
import dk.cocode.parvaz.settings.Access
import dk.cocode.parvaz.ui.onboarding.OnboardingHost
import dk.cocode.parvaz.ui.onboarding.ReadinessScreen
import dk.cocode.parvaz.ui.settings.SettingsScaffold
import dk.cocode.parvaz.ui.settings.SettingsSheet
import dk.cocode.parvaz.ui.settings.UpdateSection
import dk.cocode.parvaz.ui.theme.Paper
import dk.cocode.parvaz.update.UpdateController
import dk.cocode.parvaz.update.Version

/**
 * Top-level composable hoisted out of [MainActivity] so the activity
 * stays under the project's 200-line per-file cap. Takes plain values
 * + lambdas — no Activity reference — so it could be previewed
 * independently if a tooling pass ever wanted to.
 *
 * Routing rules:
 *   - main screen when an Access is loaded, onboarding finished, and
 *     no fresh deep-link is queued.
 *   - readiness scrim while we re-validate persisted onboarding state.
 *   - onboarding host otherwise (handles fresh-paste deep links too).
 *
 * The settings sheet is always rendered alongside the route — the gear
 * icon in [SettingsScaffold] is visible from every screen.
 */
@Composable
fun AppRoot(
    mainViewModel: MainViewModel,
    updateController: UpdateController,
    pendingParvazUrl: String?,
    pendingParvazUrlError: String?,
    activeAccess: Access?,
    onboardingComplete: Boolean,
    onboardingReadinessChecked: Boolean,
    showSettingsSheet: Boolean,
    onSettingsVisibilityChange: (Boolean) -> Unit,
    onLanguageChange: (String) -> Unit,
    onSaveAccess: (Access) -> Unit,
    onResetAccess: () -> Unit,
    onOnboardingFinished: (Access) -> Unit,
    currentLanguage: String,
) {
    Scaffold(
        modifier = Modifier
            .fillMaxSize()
            .background(Paper)
            .semantics { testTagsAsResourceId = true },
    ) { padding ->
        val hasDeepLink = pendingParvazUrl != null || pendingParvazUrlError != null
        val showMain = activeAccess != null && onboardingComplete && !hasDeepLink
        val checkingReadiness = activeAccess != null && !onboardingReadinessChecked && !hasDeepLink
        SettingsScaffold(
            onOpenSettings = { onSettingsVisibilityChange(true) },
            modifier = Modifier.padding(padding),
        ) {
            when {
                showMain -> {
                    val persianDigits = LocalConfiguration.current.locales.get(0)?.language == "fa"
                    MainScreen(
                        viewModel = mainViewModel,
                        persianNumerals = persianDigits,
                        onOpenSettings = { onSettingsVisibilityChange(true) },
                    )
                }
                checkingReadiness -> ReadinessScreen()
                else -> OnboardingHost(
                    initialDeepLinkUrl = pendingParvazUrl,
                    initialDeepLinkError = pendingParvazUrlError,
                    alreadyImportedAccess = activeAccess,
                    onLanguageChange = onLanguageChange,
                    onFinished = onOnboardingFinished,
                )
            }
        }
        if (showSettingsSheet) {
            val updateState by updateController.state.collectAsStateWithLifecycle()
            SettingsSheet(
                currentLanguage = currentLanguage,
                currentAccess = activeAccess,
                onboardingComplete = onboardingComplete,
                onLanguageChange = { newLang ->
                    onSettingsVisibilityChange(false)
                    onLanguageChange(newLang)
                },
                onSaveAccess = onSaveAccess,
                onResetAccess = {
                    onSettingsVisibilityChange(false)
                    onResetAccess()
                },
                onDismiss = { onSettingsVisibilityChange(false) },
                updateSection = {
                    UpdateSection(
                        currentVersionName = BuildConfig.VERSION_NAME,
                        state = updateState,
                        onCheck = {
                            val current = Version.parse(BuildConfig.VERSION_NAME)
                                ?: Version(0, 0, 0)
                            updateController.check(current)
                        },
                        onInstall = {
                            updateController.install(stopVpn = { mainViewModel.disconnect() })
                        },
                    )
                },
            )
        }
    }
}
