package dk.cocode.parvaz.ui.about

import androidx.annotation.StringRes
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import dk.cocode.parvaz.ui.theme.Ink
import dk.cocode.parvaz.ui.theme.InkSoft

/** A section title: a divider, then the title as a TalkBack heading. */
@Composable
internal fun SectionTitle(@StringRes title: Int) {
    Spacer(Modifier.height(12.dp))
    HorizontalDivider(color = InkSoft)
    Text(
        text = stringResource(title),
        style = MaterialTheme.typography.titleLarge,
        color = Ink,
        modifier = Modifier.semantics { heading() },
    )
}

@Composable
internal fun Body(@StringRes text: Int, soft: Boolean = false) = Text(
    text = stringResource(text),
    style = MaterialTheme.typography.bodyLarge,
    color = if (soft) InkSoft else Ink,
)

/** A full-width button whose label says what it opens. */
@Composable
internal fun LinkButton(@StringRes label: Int, tag: String, onClick: () -> Unit) {
    OutlinedButton(
        onClick = onClick,
        shape = RoundedCornerShape(2.dp),
        modifier = Modifier.fillMaxWidth().testTag(tag),
    ) {
        Text(stringResource(label), color = Ink)
    }
}

object AboutTestTags {
    const val Screen = "about_screen"
    const val Close = "about_close"
    const val Updates = "about_updates"
    const val Privacy = "about_privacy"
    const val Website = "about_website"
    const val Source = "about_source"
    const val Issues = "about_issues"
}
