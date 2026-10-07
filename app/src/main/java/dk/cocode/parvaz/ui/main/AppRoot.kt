package dk.cocode.parvaz.ui.main

import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Scaffold
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.testTagsAsResourceId
import dk.cocode.parvaz.settings.Access
import dk.cocode.parvaz.settings.AccessParseError
import dk.cocode.parvaz.ui.about.AboutScreen
import dk.cocode.parvaz.ui.onboarding.OnboardingHost
import dk.cocode.parvaz.ui.onboarding.ReadinessScreen
import dk.cocode.parvaz.ui.settings.SettingsScaffold
import dk.cocode.parvaz.ui.settings.SettingsSheet
import dk.cocode.parvaz.ui.theme.Paper

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
 * icon in [SettingsScaffold] is visible from every screen. Its "About
 * Parvaz" button opens the About page, drawn over the route (which keeps
 * its state) until the user closes it.
 */
@Composable
fun AppRoot(
    mainViewModel: MainViewModel,
    pendingParvazUrl: String?,
    pendingParvazUrlError: AccessParseError?,
    activeAccess: Access?,
    onboardingComplete: Boolean,
    onboardingReadinessChecked: Boolean,
    showSettingsSheet: Boolean,
    onSettingsVisibilityChange: (Boolean) -> Unit,
    showAbout: Boolean,
    onAboutVisibilityChange: (Boolean) -> Unit,
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
        // About is drawn over the route, which stays composed: taking the route out
        // would drop its state (a link pasted into Import, the setup step).
        // While About covers it, TalkBack must not reach what is behind.
        val behindAbout = if (showAbout) Modifier.clearAndSetSemantics { } else Modifier
        Box(Modifier.fillMaxSize()) {
            SettingsScaffold(
                onOpenSettings = { onSettingsVisibilityChange(true) },
                modifier = Modifier.padding(padding).then(behindAbout),
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
            if (showAbout) {
                AboutScreen(
                    language = currentLanguage,
                    onClose = { onAboutVisibilityChange(false) },
                    // Swallow taps so nothing behind the page can be pressed.
                    modifier = Modifier
                        .padding(padding)
                        .pointerInput(Unit) { detectTapGestures { } },
                )
            }
        }
        if (showSettingsSheet) {
            SettingsSheet(
                currentLanguage = currentLanguage,
                currentAccess = activeAccess,
                onboardingComplete = onboardingComplete,
                onLanguageChange = { newLang ->
                    onSettingsVisibilityChange(false)
                    onLanguageChange(newLang)
                },
                onSaveAccess = onSaveAccess,
                onOpenAbout = {
                    onSettingsVisibilityChange(false)
                    onAboutVisibilityChange(true)
                },
                onResetAccess = {
                    onSettingsVisibilityChange(false)
                    onResetAccess()
                },
                onDismiss = { onSettingsVisibilityChange(false) },
            )
        }
    }
}
