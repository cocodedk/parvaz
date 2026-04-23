package dk.cocode.parvaz.ui.onboarding

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import dk.cocode.parvaz.settings.Access
import dk.cocode.parvaz.ui.theme.Ink
import dk.cocode.parvaz.ui.theme.Paper

/**
 * Strictly linear onboarding: SPLASH → IMPORT → CA_INSTALL → VPN_EXPLAIN
 * → DONE. Promoted to a proper NavController when branching appears.
 */
enum class OnboardingStep { SPLASH, IMPORT, CA_INSTALL, VPN_EXPLAIN, DONE }

/**
 * Host for the onboarding flow. SPLASH and IMPORT are real; CA_INSTALL
 * and VPN_EXPLAIN are TODO placeholders until M12.3 / M12.4. DONE hands
 * off to [onFinished], which routes MainActivity into the main screen.
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

        OnboardingStep.CA_INSTALL,
        OnboardingStep.VPN_EXPLAIN -> {
            PlaceholderStep(
                label = step.name,
                onNext = {
                    step = when (step) {
                        OnboardingStep.CA_INSTALL -> OnboardingStep.VPN_EXPLAIN
                        OnboardingStep.VPN_EXPLAIN -> OnboardingStep.DONE
                        else -> OnboardingStep.DONE
                    }
                },
                modifier = modifier,
            )
        }

        OnboardingStep.DONE -> {
            val access = imported
            if (access != null) onFinished(access)
            // else: onboarding DONE with no access is a bug; main screen
            // will handle missing-access via its own fallback.
        }
    }
}

@Composable
private fun PlaceholderStep(
    label: String,
    onNext: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier
            .fillMaxSize()
            .background(Paper)
            .padding(32.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(
            text = "TODO · $label",
            style = MaterialTheme.typography.bodyLarge,
            color = Ink,
        )
        Button(
            onClick = onNext,
            colors = ButtonDefaults.buttonColors(
                containerColor = Ink,
                contentColor = Paper,
            ),
            modifier = Modifier.padding(top = 16.dp),
        ) {
            Text("NEXT", style = MaterialTheme.typography.labelLarge)
        }
    }
}
