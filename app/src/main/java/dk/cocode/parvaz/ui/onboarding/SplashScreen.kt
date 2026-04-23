package dk.cocode.parvaz.ui.onboarding

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import dk.cocode.parvaz.R
import dk.cocode.parvaz.ui.theme.Ink
import dk.cocode.parvaz.ui.theme.InkSoft
import dk.cocode.parvaz.ui.theme.Oxblood
import dk.cocode.parvaz.ui.theme.Paper
import dk.cocode.parvaz.ui.theme.ParvazTheme

/**
 * Step 1 of the onboarding flow. Title lockup + one rubber-stamp CTA
 * that starts the import flow. Pure stateless composable — the caller
 * hoists the nav action.
 */
@Composable
fun SplashScreen(
    onStart: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Box(
        modifier = modifier
            .fillMaxSize()
            .background(Paper)
            .padding(horizontal = 32.dp, vertical = 48.dp),
    ) {
        Column(
            modifier = Modifier.fillMaxSize(),
            verticalArrangement = Arrangement.Center,
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Text(
                text = stringResource(R.string.splash_title),
                style = MaterialTheme.typography.displayLarge,
                color = Ink,
            )
            Spacer(Modifier.height(24.dp))
            Text(
                text = stringResource(R.string.splash_subtitle),
                style = MaterialTheme.typography.bodyLarge,
                color = InkSoft,
            )
        }
        Button(
            onClick = onStart,
            colors = ButtonDefaults.buttonColors(
                containerColor = Oxblood,
                contentColor = Paper,
            ),
            shape = RoundedCornerShape(2.dp),
            modifier = Modifier
                .align(Alignment.BottomCenter)
                .testTag(TestTags.SplashStartButton),
        ) {
            Text(
                text = stringResource(R.string.splash_start_cta),
                style = MaterialTheme.typography.headlineMedium,
            )
        }
    }
}

object TestTags {
    const val SplashStartButton = "splash_start_button"
}

@Preview(showBackground = true, backgroundColor = 0xFFF1E8D4)
@Composable
private fun SplashPreview() {
    ParvazTheme { SplashScreen(onStart = {}) }
}
