package dk.cocode.parvaz.ui.settings

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import dk.cocode.parvaz.R
import dk.cocode.parvaz.settings.Access
import dk.cocode.parvaz.ui.theme.Ink
import dk.cocode.parvaz.ui.theme.InkSoft
import dk.cocode.parvaz.ui.theme.Paper

/**
 * Globally-reachable settings sheet — opened by the gear icon on every
 * screen. Composes four optional sections:
 *   - [UrlEditSection] — always visible. Pre-filled when an Access exists.
 *   - [LanguageSection] — always visible.
 *   - [UpdateSection]  — always visible (M-update-5).
 *   - [ResetSection]   — visible only after onboarding completes
 *     ([onboardingComplete]=true) so a user mid-CA-install can't drop
 *     themselves back to step 1 by misadventure.
 *
 * State is hoisted: the activity owns the open/closed flag, the current
 * Access, and persists URL/language changes through ParvazSettings.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SettingsSheet(
    currentLanguage: String,
    currentAccess: Access?,
    onboardingComplete: Boolean,
    onLanguageChange: (String) -> Unit,
    onSaveAccess: (Access) -> Unit,
    onResetAccess: () -> Unit,
    onDismiss: () -> Unit,
    updateSection: @Composable () -> Unit = {},
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        containerColor = Paper,
        modifier = Modifier.testTag(SettingsTestTags.Sheet),
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 24.dp, vertical = 16.dp),
            verticalArrangement = Arrangement.spacedBy(20.dp),
        ) {
            Text(
                text = stringResource(R.string.settings_title),
                style = MaterialTheme.typography.headlineMedium,
                color = Ink,
            )

            UrlEditSection(currentAccess = currentAccess, onSave = onSaveAccess)
            HorizontalDivider(color = InkSoft)

            LanguageSection(currentLanguage = currentLanguage, onLanguageChange = onLanguageChange)
            HorizontalDivider(color = InkSoft)

            updateSection()

            if (onboardingComplete) {
                HorizontalDivider(color = InkSoft)
                ResetSection(onResetAccess = onResetAccess)
            }
            Spacer(Modifier.height(16.dp))
        }
    }
}
