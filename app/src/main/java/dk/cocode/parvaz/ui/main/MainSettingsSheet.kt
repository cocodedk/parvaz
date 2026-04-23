package dk.cocode.parvaz.ui.main

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import dk.cocode.parvaz.R
import dk.cocode.parvaz.ui.theme.Ink
import dk.cocode.parvaz.ui.theme.InkSoft
import dk.cocode.parvaz.ui.theme.Oxblood
import dk.cocode.parvaz.ui.theme.Paper

/**
 * Bottom sheet triggered by long-pressing the main-screen stamp.
 * Two actions in M13b:
 *   - Language toggle (fa/en) — written to [onLanguageChange]; the
 *     activity re-reads [ParvazSettings.language] through an
 *     `attachBaseContext` wrapper and recreate() applies it.
 *   - Access reset — clears saved Access + onboarding flag, routes
 *     back through [OnboardingHost]. Gated by a confirmation dialog
 *     because it's destructive.
 *
 * SNI pool override is deferred — the sidecar plumbing to consume a
 * user-set list doesn't exist yet.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun MainSettingsSheet(
    currentLanguage: String,
    onLanguageChange: (String) -> Unit,
    onResetAccess: () -> Unit,
    onDismiss: () -> Unit,
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    var showResetConfirm by remember { mutableStateOf(false) }

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        containerColor = Paper,
    ) {
        Column(
            modifier = Modifier.fillMaxWidth().padding(horizontal = 24.dp, vertical = 16.dp),
            verticalArrangement = Arrangement.spacedBy(20.dp),
        ) {
            Text(
                text = stringResource(R.string.settings_title),
                style = MaterialTheme.typography.headlineMedium,
                color = Ink,
            )

            LanguageSection(currentLanguage, onLanguageChange)
            ResetAccessSection(onRequestReset = { showResetConfirm = true })
            Spacer(Modifier.height(16.dp))
        }
    }

    if (showResetConfirm) {
        ResetConfirmDialog(
            onConfirm = {
                showResetConfirm = false
                onResetAccess()
            },
            onCancel = { showResetConfirm = false },
        )
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun LanguageSection(currentLanguage: String, onLanguageChange: (String) -> Unit) {
    val options = listOf(
        "fa" to stringResource(R.string.settings_language_fa),
        "en" to stringResource(R.string.settings_language_en),
    )
    Text(
        text = stringResource(R.string.settings_language_label),
        style = MaterialTheme.typography.titleMedium,
        color = InkSoft,
    )
    SingleChoiceSegmentedButtonRow(modifier = Modifier.fillMaxWidth()) {
        options.forEachIndexed { index, (code, label) ->
            SegmentedButton(
                selected = code == currentLanguage,
                onClick = { if (code != currentLanguage) onLanguageChange(code) },
                shape = SegmentedButtonDefaults.itemShape(index, options.size),
            ) {
                Text(label)
            }
        }
    }
}

@Composable
private fun ResetAccessSection(onRequestReset: () -> Unit) {
    Text(
        text = stringResource(R.string.settings_reset_access_label),
        style = MaterialTheme.typography.titleMedium,
        color = InkSoft,
    )
    Text(
        text = stringResource(R.string.settings_reset_access_body),
        style = MaterialTheme.typography.bodyMedium,
        color = InkSoft,
    )
    OutlinedButton(
        onClick = onRequestReset,
        shape = RoundedCornerShape(2.dp),
        modifier = Modifier.fillMaxWidth(),
    ) {
        Text(stringResource(R.string.settings_reset_access_cta), color = Oxblood)
    }
}

@Composable
private fun ResetConfirmDialog(onConfirm: () -> Unit, onCancel: () -> Unit) {
    AlertDialog(
        onDismissRequest = onCancel,
        containerColor = Paper,
        title = {
            Text(
                text = stringResource(R.string.settings_reset_confirm_title),
                color = Ink,
            )
        },
        text = {
            Text(
                text = stringResource(R.string.settings_reset_confirm_body),
                color = InkSoft,
            )
        },
        confirmButton = {
            Button(
                onClick = onConfirm,
                colors = ButtonDefaults.buttonColors(containerColor = Oxblood, contentColor = Paper),
                shape = RoundedCornerShape(2.dp),
            ) {
                Text(stringResource(R.string.settings_reset_access_cta))
            }
        },
        dismissButton = {
            OutlinedButton(onClick = onCancel, shape = RoundedCornerShape(2.dp)) {
                Text(stringResource(R.string.settings_cancel_cta), color = Ink)
            }
        },
    )
}
