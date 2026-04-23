package dk.cocode.parvaz.ui.onboarding

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import dk.cocode.parvaz.settings.Access

/**
 * Strictly linear onboarding: SPLASH → IMPORT → CA_INSTALL → VPN_EXPLAIN
 * → DONE. Promoted to a proper NavController when branching appears.
 */
enum class OnboardingStep { SPLASH, IMPORT, CA_INSTALL, VPN_EXPLAIN, DONE }

/**
 * Host for the onboarding flow. All four steps are live (M12.1–M12.4).
 * DONE hands off to [onFinished], which routes MainActivity into the
 * main screen (M13).
 *
 * [initialDeepLinkUrl] / [initialDeepLinkError] pre-fill the Import
 * screen when the user arrived via a \`parvaz://\` deep link — the Splash
 * step is still shown so the flow feels consistent.
 *
 * [alreadyImportedAccess], when non-null, means the user has an Access
 * persisted from a prior session. With no pending deep link we jump
 * straight to CA_INSTALL (the next uncompleted step) so the user isn't
 * asked to re-import every launch. A fresh deep link overrides this and
 * routes through IMPORT normally.
 */
@Composable
fun OnboardingHost(
    initialDeepLinkUrl: String? = null,
    initialDeepLinkError: String? = null,
    alreadyImportedAccess: Access? = null,
    onFinished: (Access) -> Unit,
    modifier: Modifier = Modifier,
) {
    val startStep = when {
        initialDeepLinkUrl != null || initialDeepLinkError != null -> OnboardingStep.IMPORT
        alreadyImportedAccess != null -> OnboardingStep.CA_INSTALL
        else -> OnboardingStep.SPLASH
    }
    var step by rememberSaveable { mutableStateOf(startStep) }
    // `Access` has no Saver; on recreation we re-derive from the caller's
    // alreadyImportedAccess, which MainActivity rereads from settings.
    var imported by remember { mutableStateOf(alreadyImportedAccess) }

    when (step) {
        OnboardingStep.SPLASH ->
            SplashScreen(
                onStart = { step = OnboardingStep.IMPORT },
                modifier = modifier,
            )

        OnboardingStep.IMPORT ->
            ImportAccessScreen(
                initialUrl = initialDeepLinkUrl,
                initialError = initialDeepLinkError,
                onImported = { access ->
                    imported = access
                    step = OnboardingStep.CA_INSTALL
                },
                modifier = modifier,
            )

        OnboardingStep.CA_INSTALL ->
            CaInstallScreen(
                onInstalled = { step = OnboardingStep.VPN_EXPLAIN },
                modifier = modifier,
            )

        OnboardingStep.VPN_EXPLAIN ->
            VpnPermissionScreen(
                onGranted = { step = OnboardingStep.DONE },
                modifier = modifier,
            )

        OnboardingStep.DONE -> {
            val access = imported
            if (access != null) onFinished(access)
            // else: onboarding DONE with no access is a bug; main screen
            // will handle missing-access via its own fallback.
        }
    }
}
