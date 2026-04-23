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
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import dk.cocode.parvaz.ui.theme.Ink
import dk.cocode.parvaz.ui.theme.Paper

/**
 * Strictly linear onboarding: SPLASH → IMPORT → CA_INSTALL → VPN_EXPLAIN
 * → DONE. Promoted to a proper NavController when branching appears.
 */
enum class OnboardingStep { SPLASH, IMPORT, CA_INSTALL, VPN_EXPLAIN, DONE }

/**
 * Host for the onboarding flow. Only SPLASH is real in M12.1; the next
 * three steps are placeholder "tap to advance" panels that land as their
 * own PRs (M12.2/3/4). DONE hands off to [onFinished], which routes
 * MainActivity into the rest of the app.
 */
@Composable
fun OnboardingHost(
    onFinished: () -> Unit,
    modifier: Modifier = Modifier,
) {
    var step by remember { mutableStateOf(OnboardingStep.SPLASH) }

    when (step) {
        OnboardingStep.SPLASH ->
            SplashScreen(
                onStart = { step = OnboardingStep.IMPORT },
                modifier = modifier,
            )

        OnboardingStep.IMPORT,
        OnboardingStep.CA_INSTALL,
        OnboardingStep.VPN_EXPLAIN -> {
            PlaceholderStep(
                label = step.name,
                onNext = {
                    step = when (step) {
                        OnboardingStep.IMPORT -> OnboardingStep.CA_INSTALL
                        OnboardingStep.CA_INSTALL -> OnboardingStep.VPN_EXPLAIN
                        OnboardingStep.VPN_EXPLAIN -> OnboardingStep.DONE
                        else -> OnboardingStep.DONE
                    }
                },
                modifier = modifier,
            )
        }

        OnboardingStep.DONE -> onFinished()
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
