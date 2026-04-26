package dk.cocode.parvaz.ui.onboarding

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import dk.cocode.parvaz.ui.theme.Ink
import dk.cocode.parvaz.ui.theme.Paper

/**
 * Transitional screen shown while we re-check the device for installed
 * CA + granted VpnService permission on cold start. Should usually be
 * visible for only a frame or two; render the NOTAM-paper background
 * so the splash → main handoff stays visually quiet.
 */
@Composable
fun ReadinessScreen(modifier: Modifier = Modifier) {
    Box(
        modifier = modifier
            .fillMaxSize()
            .background(Paper),
        contentAlignment = Alignment.Center,
    ) {
        CircularProgressIndicator(color = Ink)
    }
}
