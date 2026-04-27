package dk.cocode.parvaz.ui.settings

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import dk.cocode.parvaz.R
import dk.cocode.parvaz.ui.theme.Ink
import dk.cocode.parvaz.ui.theme.InkSoft
import dk.cocode.parvaz.ui.theme.Olive
import dk.cocode.parvaz.ui.theme.Oxblood
import dk.cocode.parvaz.ui.theme.Paper
import dk.cocode.parvaz.update.UpdateState

/**
 * Renders the update section of the settings sheet. State machine is
 * driven by the caller (an [UpdateController] instance held by the
 * activity) — this composable is pure presentation.
 */
@Composable
fun UpdateSection(
    currentVersionName: String,
    state: UpdateState,
    onCheck: () -> Unit,
    onInstall: () -> Unit,
) {
    Column(
        modifier = Modifier.fillMaxWidth(),
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        Text(
            text = stringResource(R.string.settings_update_section_label),
            style = MaterialTheme.typography.titleMedium,
            color = InkSoft,
        )
        Text(
            text = stringResource(R.string.settings_update_current_version, currentVersionName),
            style = MaterialTheme.typography.bodySmall,
            color = InkSoft,
        )

        when (state) {
            UpdateState.Idle -> CheckButton(onCheck)
            UpdateState.Checking -> StatusLabel(R.string.settings_update_checking)
            UpdateState.UpToDate -> {
                StatusLabel(R.string.settings_update_uptodate, color = Olive)
                CheckButton(onCheck)
            }
            is UpdateState.Available -> {
                Text(
                    text = stringResource(R.string.settings_update_available, state.release.tagName),
                    style = MaterialTheme.typography.bodyLarge,
                    color = Ink,
                    modifier = Modifier.testTag(SettingsTestTags.UpdateStatusText),
                )
                ReleaseNotesBlock(state.release.body)
                InstallButton(onInstall)
            }
            is UpdateState.Disconnecting -> StatusLabel(R.string.settings_update_disconnecting)
            is UpdateState.Downloading -> {
                val pct = if (state.totalBytes > 0) {
                    ((state.downloadedBytes * 100) / state.totalBytes).toInt().coerceIn(0, 100)
                } else 0
                Text(
                    text = stringResource(R.string.settings_update_downloading, pct),
                    style = MaterialTheme.typography.bodyMedium,
                    color = Ink,
                    modifier = Modifier.testTag(SettingsTestTags.UpdateStatusText),
                )
                LinearProgressIndicator(
                    progress = { pct / 100f },
                    modifier = Modifier.fillMaxWidth(),
                )
            }
            is UpdateState.InstallerHandoff ->
                StatusLabel(R.string.settings_update_installer_handoff, color = Olive)
            is UpdateState.NeedsUnknownSources -> {
                StatusLabel(R.string.settings_update_error_unknown_sources, color = Oxblood)
                InstallButton(onInstall)
            }
            UpdateState.Failure.Network -> {
                StatusLabel(R.string.settings_update_error_network, color = Oxblood)
                CheckButton(onCheck)
            }
            UpdateState.Failure.NoAsset -> {
                StatusLabel(R.string.settings_update_error_no_asset, color = Oxblood)
                CheckButton(onCheck)
            }
            UpdateState.Failure.ShaMismatch -> {
                StatusLabel(R.string.settings_update_error_sha, color = Oxblood)
                CheckButton(onCheck)
            }
        }
    }
}

@Composable
private fun CheckButton(onClick: () -> Unit) {
    OutlinedButton(
        onClick = onClick,
        shape = RoundedCornerShape(2.dp),
        modifier = Modifier
            .fillMaxWidth()
            .testTag(SettingsTestTags.UpdateCheckButton),
    ) {
        Text(stringResource(R.string.settings_update_check_cta), color = Ink)
    }
}

@Composable
private fun InstallButton(onClick: () -> Unit) {
    Button(
        onClick = onClick,
        colors = ButtonDefaults.buttonColors(containerColor = Olive, contentColor = Paper),
        shape = RoundedCornerShape(2.dp),
        modifier = Modifier
            .fillMaxWidth()
            .testTag(SettingsTestTags.UpdateInstallButton),
    ) {
        Text(stringResource(R.string.settings_update_install_cta))
    }
}

@Composable
private fun StatusLabel(resId: Int, color: androidx.compose.ui.graphics.Color = InkSoft) {
    Text(
        text = stringResource(resId),
        style = MaterialTheme.typography.bodyMedium,
        color = color,
        modifier = Modifier.testTag(SettingsTestTags.UpdateStatusText),
    )
}

@Composable
private fun ReleaseNotesBlock(body: String) {
    if (body.isBlank()) return
    Text(
        text = stringResource(R.string.settings_update_release_notes),
        style = MaterialTheme.typography.titleSmall,
        color = InkSoft,
    )
    Text(
        text = body.trim(),
        style = MaterialTheme.typography.bodySmall,
        color = Ink,
        modifier = Modifier.padding(top = 4.dp),
    )
}
