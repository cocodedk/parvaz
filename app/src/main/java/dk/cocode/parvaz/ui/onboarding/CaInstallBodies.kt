package dk.cocode.parvaz.ui.onboarding

import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import dk.cocode.parvaz.R
import dk.cocode.parvaz.ui.theme.Burnt
import dk.cocode.parvaz.ui.theme.Ink
import dk.cocode.parvaz.ui.theme.InkSoft
import dk.cocode.parvaz.ui.theme.Olive
import dk.cocode.parvaz.ui.theme.Oxblood
import dk.cocode.parvaz.ui.theme.Paper

@Composable
internal fun CaInstallHeader(phase: CaInstallPhase) {
    val titleRes = if (phase == CaInstallPhase.NO_SCREEN_LOCK)
        R.string.ca_install_no_lock_title else R.string.ca_install_title
    val bodyRes = when (phase) {
        CaInstallPhase.NO_SCREEN_LOCK -> R.string.ca_install_no_lock_body
        CaInstallPhase.FAILED -> R.string.ca_install_failed
        CaInstallPhase.UNVERIFIED -> R.string.ca_install_unverified_body
        CaInstallPhase.GENERATING, CaInstallPhase.AWAITING_INSTALL -> R.string.ca_install_generating
        CaInstallPhase.VERIFYING -> R.string.ca_install_verifying
        CaInstallPhase.INSTALLED -> R.string.ca_install_done_label
        CaInstallPhase.READY -> R.string.ca_install_body
    }
    val bodyColor = when (phase) {
        CaInstallPhase.NO_SCREEN_LOCK, CaInstallPhase.UNVERIFIED -> Burnt
        CaInstallPhase.FAILED -> Oxblood
        CaInstallPhase.INSTALLED -> Olive
        else -> InkSoft
    }
    Text(stringResource(titleRes), style = MaterialTheme.typography.displayMedium, color = Ink)
    Text(stringResource(bodyRes), style = MaterialTheme.typography.bodyMedium, color = bodyColor)
}

@Composable
internal fun CaInstallPrimary(phase: CaInstallPhase, onClick: () -> Unit) {
    val spinning = phase == CaInstallPhase.GENERATING ||
        phase == CaInstallPhase.AWAITING_INSTALL ||
        phase == CaInstallPhase.VERIFYING
    if (spinning) {
        CircularProgressIndicator(color = Ink, modifier = Modifier.testTag(TestTags.CaInstallSpinner))
        return
    }
    if (phase == CaInstallPhase.NO_SCREEN_LOCK || phase == CaInstallPhase.INSTALLED) return
    val (ctaRes, container) = when (phase) {
        CaInstallPhase.READY -> R.string.ca_install_cta to Oxblood
        CaInstallPhase.FAILED,
        CaInstallPhase.UNVERIFIED -> R.string.ca_install_retry_cta to Oxblood
        else -> return
    }
    Button(
        onClick = onClick,
        colors = ButtonDefaults.buttonColors(containerColor = container, contentColor = Paper),
        shape = RoundedCornerShape(2.dp),
        modifier = Modifier.fillMaxWidth().testTag(TestTags.CaInstallPrimary),
    ) {
        Text(stringResource(ctaRes), style = MaterialTheme.typography.headlineMedium)
    }
}

/**
 * Secondary "Continue anyway" button, shown only on UNVERIFIED. Gives
 * the user an out when we can't automatically confirm CA install —
 * typical on Samsung / Android 14, where AndroidCAStore hides user CAs
 * from unprivileged app processes. If the cert isn't actually trusted,
 * failure surfaces at connect-time via the browser instead.
 */
@Composable
internal fun CaInstallContinue(phase: CaInstallPhase, onContinue: () -> Unit) {
    if (phase != CaInstallPhase.UNVERIFIED) return
    Button(
        onClick = onContinue,
        colors = ButtonDefaults.buttonColors(containerColor = Olive, contentColor = Paper),
        shape = RoundedCornerShape(2.dp),
        modifier = Modifier.fillMaxWidth().testTag(TestTags.CaInstallContinue),
    ) {
        Text(
            stringResource(R.string.ca_install_continue_cta),
            style = MaterialTheme.typography.headlineMedium,
        )
    }
}
