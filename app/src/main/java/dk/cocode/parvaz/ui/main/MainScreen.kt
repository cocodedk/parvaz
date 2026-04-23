package dk.cocode.parvaz.ui.main

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import dk.cocode.parvaz.R
import dk.cocode.parvaz.ui.theme.Ink
import dk.cocode.parvaz.ui.theme.InkSoft
import dk.cocode.parvaz.ui.theme.Olive
import dk.cocode.parvaz.ui.theme.Oxblood
import dk.cocode.parvaz.ui.theme.Paper
import dk.cocode.parvaz.ui.util.formatUptime
import dk.cocode.parvaz.vpn.ParvazVpnService.ConnectionState

/**
 * Post-onboarding main screen. One big rubber-stamp button:
 *   - DISCONNECTED → outlined oxblood "پرواز" stamp. Tap connects.
 *   - CONNECTING → spinner + "در حال اتصال…".
 *   - CONNECTED → solid olive "در پرواز" stamp + uptime T+HH:MM:SS.
 *     Tap again disconnects — no confirmation (one-button UX).
 *   - FAILED → oxblood error text, same tap-to-retry semantics.
 *
 * Long-press anywhere outside the stamp opens the hidden settings
 * sheet via [onOpenSettings] (M13b). Short taps continue to reach the
 * stamp button — Compose's hierarchical hit-test lets the gesture
 * detector at the column level observe events without consuming the
 * pointer stream.
 *
 * Persian numerals are opt-in via [persianNumerals]; caller sets it
 * from the UI language preference.
 */
@Composable
fun MainScreen(
    viewModel: MainViewModel,
    persianNumerals: Boolean = true,
    onOpenSettings: () -> Unit = {},
    modifier: Modifier = Modifier,
) {
    val ui by viewModel.ui.collectAsStateWithLifecycle()
    Column(
        modifier = modifier
            .fillMaxSize()
            .background(Paper)
            .pointerInput(Unit) {
                detectTapGestures(onLongPress = { onOpenSettings() })
            }
            .padding(32.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        when (ui.phase) {
            ConnectionState.DISCONNECTED, ConnectionState.FAILED -> DisconnectedStamp(
                failed = ui.phase == ConnectionState.FAILED,
                onClick = { viewModel.connect() },
            )
            ConnectionState.CONNECTING -> ConnectingIndicator()
            ConnectionState.CONNECTED -> ConnectedStamp(
                uptimeSeconds = ui.uptimeSeconds,
                persian = persianNumerals,
                onClick = { viewModel.disconnect() },
            )
        }
    }
}

@Composable
private fun DisconnectedStamp(failed: Boolean, onClick: () -> Unit) {
    OutlinedButton(
        onClick = onClick,
        border = BorderStroke(3.dp, Oxblood),
        shape = RoundedCornerShape(4.dp),
        modifier = Modifier.size(width = 260.dp, height = 110.dp),
    ) {
        Text(
            text = stringResource(R.string.main_disconnected_stamp),
            style = MaterialTheme.typography.displayMedium,
            color = Oxblood,
        )
    }
    if (failed) {
        Spacer(Modifier.height(16.dp))
        Text(
            text = stringResource(R.string.main_failed_label),
            style = MaterialTheme.typography.bodyMedium,
            color = Oxblood,
        )
    }
}

@Composable
private fun ConnectingIndicator() {
    Box(contentAlignment = Alignment.Center, modifier = Modifier.size(110.dp)) {
        CircularProgressIndicator(color = Ink)
    }
    Spacer(Modifier.height(24.dp))
    Text(
        text = stringResource(R.string.main_connecting_label),
        style = MaterialTheme.typography.bodyLarge,
        color = InkSoft,
    )
}

@Composable
private fun ConnectedStamp(uptimeSeconds: Long, persian: Boolean, onClick: () -> Unit) {
    Button(
        onClick = onClick,
        colors = ButtonDefaults.buttonColors(containerColor = Olive, contentColor = Paper),
        shape = RoundedCornerShape(4.dp),
        modifier = Modifier.size(width = 260.dp, height = 110.dp),
    ) {
        Text(
            text = stringResource(R.string.main_connected_stamp),
            style = MaterialTheme.typography.displayMedium,
            color = Color.Unspecified,
        )
    }
    Spacer(Modifier.height(20.dp))
    Text(
        text = formatUptime(uptimeSeconds, persian = persian),
        style = MaterialTheme.typography.headlineMedium,
        color = Olive,
    )
}
