package dk.cocode.parvaz.ui.onboarding

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import dk.cocode.parvaz.R
import dk.cocode.parvaz.settings.ParvazSettings
import dk.cocode.parvaz.ui.theme.InkSoft

/**
 * "Switch to <other language>" affordance pinned to every onboarding
 * surface — without it, users on English-locale devices land on the
 * Farsi-default flow with no escape hatch until they reach the main
 * screen's hidden settings sheet. The label always names the OTHER
 * language so the affordance reads as "tap = get this".
 */
@Composable
fun LanguageToggle(
    onLanguageChange: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    val context = LocalContext.current
    val current = remember { ParvazSettings(context).language }
    val target = if (current == "fa") "en" else "fa"
    val labelRes = if (current == "fa") R.string.settings_language_en else R.string.settings_language_fa
    TextButton(
        onClick = { onLanguageChange(target) },
        modifier = modifier.testTag(TestTags.LanguageToggle),
    ) {
        Text(
            text = stringResource(labelRes),
            color = InkSoft,
            style = MaterialTheme.typography.bodyMedium,
        )
    }
}
