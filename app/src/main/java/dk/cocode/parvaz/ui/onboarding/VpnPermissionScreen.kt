package dk.cocode.parvaz.ui.onboarding

import android.app.Activity
import android.content.Intent
import android.net.VpnService
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts.StartActivityForResult
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalLifecycleOwner
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import dk.cocode.parvaz.R
import dk.cocode.parvaz.ui.theme.Ink
import dk.cocode.parvaz.ui.theme.InkSoft
import dk.cocode.parvaz.ui.theme.Olive
import dk.cocode.parvaz.ui.theme.Oxblood
import dk.cocode.parvaz.ui.theme.Paper
import kotlinx.coroutines.delay

enum class VpnPermissionPhase { IDLE, AWAITING_SYSTEM_PROMPT, GRANTED, DENIED }

/**
 * Step 4 of onboarding — the last one. Farsi explainer BEFORE Android's
 * system VPN consent dialog, so the user has context for what they're
 * about to approve.
 *
 *  1. Enter → `VpnService.prepare(context)`. null means already granted,
 *     jump to GRANTED. Otherwise stay IDLE with the CTA live.
 *  2. Tap CTA → launch the system intent, phase = AWAITING_SYSTEM_PROMPT.
 *  3. Activity result: RESULT_OK → GRANTED, anything else → DENIED.
 *  4. GRANTED briefly visible, then [onGranted] — the onboarding host
 *     advances to DONE and MainActivity owns what happens next.
 *
 * `phase` is rememberSaveable so rotation / process death resume cleanly.
 * `notified` stays in-memory only so a recreated composition re-fires
 * [onGranted] if the 400ms celebrate-delay was interrupted (same lesson
 * the M12.3 review surfaced).
 */
@Composable
fun VpnPermissionScreen(
    onGranted: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val context = LocalContext.current
    var phase by rememberSaveable { mutableStateOf(VpnPermissionPhase.IDLE) }
    var notified by remember { mutableStateOf(false) }

    val launcher = rememberLauncherForActivityResult(StartActivityForResult()) { result ->
        phase = if (result.resultCode == Activity.RESULT_OK) {
            VpnPermissionPhase.GRANTED
        } else {
            VpnPermissionPhase.DENIED
        }
    }

    LaunchedEffect(Unit) {
        // If already granted from a prior run, skip straight through.
        if (phase == VpnPermissionPhase.IDLE && VpnService.prepare(context) == null) {
            phase = VpnPermissionPhase.GRANTED
        }
    }

    LaunchedEffect(phase, notified) {
        if (phase == VpnPermissionPhase.GRANTED && !notified) {
            notified = true
            delay(400)
            onGranted()
        }
    }

    // Recovery: if the activity is recreated while on the system VPN
    // dialog (process death, user backgrounded and returned), the
    // launcher callback may never fire. On ON_RESUME, if we're still
    // "awaiting" but permission isn't granted, fall back to IDLE so
    // the user can retry.
    val lifecycleOwner = LocalLifecycleOwner.current
    DisposableEffect(lifecycleOwner) {
        val observer = LifecycleEventObserver { _, event ->
            if (event == Lifecycle.Event.ON_RESUME &&
                phase == VpnPermissionPhase.AWAITING_SYSTEM_PROMPT &&
                VpnService.prepare(context) != null
            ) {
                phase = VpnPermissionPhase.IDLE
            }
        }
        lifecycleOwner.lifecycle.addObserver(observer)
        onDispose { lifecycleOwner.lifecycle.removeObserver(observer) }
    }

    val request = {
        val intent: Intent? = VpnService.prepare(context)
        if (intent == null) {
            // System says already granted (race with external change).
            phase = VpnPermissionPhase.GRANTED
        } else {
            phase = VpnPermissionPhase.AWAITING_SYSTEM_PROMPT
            launcher.launch(intent)
        }
    }

    Column(
        modifier = modifier
            .fillMaxSize()
            .background(Paper)
            .padding(horizontal = 32.dp, vertical = 48.dp),
        verticalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        VpnPermissionHeader(phase)
        Spacer(Modifier.height(8.dp))
        VpnPermissionPrimary(phase, onClick = request)
    }
}

@Composable
private fun VpnPermissionHeader(phase: VpnPermissionPhase) {
    val bodyRes = when (phase) {
        VpnPermissionPhase.IDLE -> R.string.vpn_explain_body
        VpnPermissionPhase.AWAITING_SYSTEM_PROMPT -> R.string.vpn_explain_waiting
        VpnPermissionPhase.GRANTED -> R.string.vpn_explain_granted
        VpnPermissionPhase.DENIED -> R.string.vpn_explain_denied
    }
    val bodyColor = when (phase) {
        VpnPermissionPhase.DENIED -> Oxblood
        VpnPermissionPhase.GRANTED -> Olive
        else -> InkSoft
    }
    Text(
        text = stringResource(R.string.vpn_explain_title),
        style = MaterialTheme.typography.displayMedium,
        color = Ink,
    )
    Text(
        text = stringResource(bodyRes),
        style = MaterialTheme.typography.bodyMedium,
        color = bodyColor,
    )
}

@Composable
private fun VpnPermissionPrimary(phase: VpnPermissionPhase, onClick: () -> Unit) {
    if (phase == VpnPermissionPhase.AWAITING_SYSTEM_PROMPT) {
        CircularProgressIndicator(
            color = Ink,
            modifier = Modifier.testTag(TestTags.VpnPermissionSpinner),
        )
        return
    }
    if (phase == VpnPermissionPhase.GRANTED) return
    val ctaRes = when (phase) {
        VpnPermissionPhase.IDLE -> R.string.vpn_explain_cta
        VpnPermissionPhase.DENIED -> R.string.ca_install_retry_cta
        else -> return
    }
    Button(
        onClick = onClick,
        colors = ButtonDefaults.buttonColors(containerColor = Oxblood, contentColor = Paper),
        shape = RoundedCornerShape(2.dp),
        modifier = Modifier.fillMaxWidth().testTag(TestTags.VpnPermissionPrimary),
    ) {
        Text(
            text = stringResource(ctaRes),
            style = MaterialTheme.typography.headlineMedium,
        )
    }
}
