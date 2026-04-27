package dk.cocode.parvaz.ui.settings

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import dk.cocode.parvaz.R
import dk.cocode.parvaz.ui.theme.Ink

/**
 * Wraps any top-level screen with a globally-visible settings gear at
 * the top-start corner of the viewport. The gear is intentionally the
 * only chrome in this scaffold — onboarding screens already manage
 * their own chrome (language toggle, step counters), and the project
 * design language reserves a full TopAppBar for screens that earn one.
 *
 * The gear sits at TopStart (top-left in LTR, top-right in RTL) so it
 * never collides with the existing language toggle pinned at TopEnd
 * inside [OnboardingHost].
 *
 * State for "is the settings sheet open?" is hoisted to the caller —
 * MainActivity owns the flag and renders the sheet itself, so the
 * scaffold only emits a click event.
 */
@Composable
fun SettingsScaffold(
    onOpenSettings: () -> Unit,
    modifier: Modifier = Modifier,
    content: @Composable () -> Unit,
) {
    Box(modifier = modifier.fillMaxSize()) {
        content()
        IconButton(
            onClick = onOpenSettings,
            modifier = Modifier
                .align(Alignment.TopStart)
                .padding(horizontal = 4.dp, vertical = 4.dp)
                .size(44.dp)
                .testTag(SettingsTestTags.Gear),
        ) {
            Icon(
                painter = painterResource(R.drawable.ic_settings_gear),
                contentDescription = stringResource(R.string.settings_gear_cd),
                tint = Ink,
            )
        }
    }
}

object SettingsTestTags {
    const val Gear = "settings_gear"
    const val Sheet = "settings_sheet"
    const val UrlField = "settings_url_field"
    const val UrlSaveButton = "settings_url_save"
    const val UrlError = "settings_url_error"
    const val ResetButton = "settings_reset_button"
    const val UpdateCheckButton = "settings_update_check"
    const val UpdateInstallButton = "settings_update_install"
    const val UpdateStatusText = "settings_update_status"
}
