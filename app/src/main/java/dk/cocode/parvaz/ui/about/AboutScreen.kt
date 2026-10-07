package dk.cocode.parvaz.ui.about

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import dk.cocode.parvaz.R
import dk.cocode.parvaz.ui.theme.Ink
import dk.cocode.parvaz.ui.theme.Oxblood
import dk.cocode.parvaz.ui.theme.Paper

/**
 * The About page, in the order the cocode-apps standard asks for: name and version with the update
 * button, what Parvaz does, privacy, links, credits and licenses, made by Cocode, and an empty
 * slot for Support. Every section title is a TalkBack heading. [language] picks the site page.
 * The page checks nothing over the network: the update button only opens a web page.
 */
@Composable
fun AboutScreen(
    language: String,
    onClose: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val context = LocalContext.current
    val version = remember { appVersionName(context) }
    var noBrowser by rememberSaveable { mutableStateOf(false) }
    val open = { link: AboutLink ->
        aboutUrl(link, language)?.let { noBrowser = !openUrl(context, it) }
        Unit
    }
    BackHandler(onBack = onClose)

    Column(
        modifier = modifier
            .fillMaxSize()
            .background(Paper)
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 24.dp, vertical = 16.dp)
            .testTag(AboutTestTags.Screen),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        OutlinedButton(
            onClick = onClose,
            shape = RoundedCornerShape(2.dp),
            modifier = Modifier.testTag(AboutTestTags.Close),
        ) {
            Text(stringResource(R.string.about_close_cta), color = Oxblood)
        }
        Text(
            text = stringResource(R.string.about_title),
            style = MaterialTheme.typography.headlineMedium,
            color = Ink,
            modifier = Modifier.semantics { heading() },
        )

        SectionTitle(R.string.about_name_version_title)
        Text(
            text = stringResource(R.string.app_name),
            style = MaterialTheme.typography.titleLarge,
            color = Ink,
        )
        Text(
            text = stringResource(R.string.about_version, version),
            style = MaterialTheme.typography.bodyLarge,
            color = Ink,
        )
        Body(R.string.about_updates_hint, soft = true)
        LinkButton(R.string.about_check_updates, AboutTestTags.Updates) { open(AboutLink.Updates) }

        SectionTitle(R.string.about_what_title)
        Body(R.string.about_what_body)

        SectionTitle(R.string.about_privacy_title)
        Body(R.string.about_privacy_relay)
        Body(R.string.about_privacy_cocode)
        // Left out when the privacy policy has no address (see PRIVACY_URL).
        if (aboutUrl(AboutLink.Privacy, language) != null) {
            LinkButton(R.string.about_privacy_link, AboutTestTags.Privacy) { open(AboutLink.Privacy) }
        }

        SectionTitle(R.string.about_links_title)
        LinkButton(R.string.about_website, AboutTestTags.Website) { open(AboutLink.Website) }
        LinkButton(R.string.about_source, AboutTestTags.Source) { open(AboutLink.Source) }
        Body(R.string.about_report_hint, soft = true)
        LinkButton(R.string.about_report, AboutTestTags.Issues) { open(AboutLink.Issues) }
        if (noBrowser) {
            Text(
                text = stringResource(R.string.about_no_browser),
                style = MaterialTheme.typography.bodyLarge,
                color = Oxblood,
            )
        }

        SectionTitle(R.string.about_credits_title)
        Body(R.string.about_license)
        Body(R.string.about_credit_upstream, soft = true)
        Body(R.string.about_credit_tun2socks, soft = true)
        Body(R.string.about_credit_androidx, soft = true)

        SectionTitle(R.string.about_made_by)

        // Support slot: intentionally empty until the Support phase (cocode-apps standard/support.md).
    }
}
