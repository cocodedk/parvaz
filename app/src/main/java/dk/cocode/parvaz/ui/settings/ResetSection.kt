package dk.cocode.parvaz.ui.settings

import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import dk.cocode.parvaz.R
import dk.cocode.parvaz.ui.theme.Ink
import dk.cocode.parvaz.ui.theme.InkSoft
import dk.cocode.parvaz.ui.theme.Oxblood
import dk.cocode.parvaz.ui.theme.Paper

/**
 * Destructive "wipe everything" entry. Caller decides whether to render
 * this at all — onboarding-incomplete state hides the section entirely
 * so a user mid-CA-install can't drop themselves back to step 1 by
 * misadventure. The confirm dialog is the second guard.
 */
@Composable
fun ResetSection(onResetAccess: () -> Unit) {
    var showResetConfirm by remember { mutableStateOf(false) }

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
        onClick = { showResetConfirm = true },
        shape = RoundedCornerShape(2.dp),
        modifier = Modifier
            .fillMaxWidth()
            .testTag(SettingsTestTags.ResetButton),
    ) {
        Text(stringResource(R.string.settings_reset_access_cta), color = Oxblood)
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
