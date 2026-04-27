package dk.cocode.parvaz.ui.settings

import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import dk.cocode.parvaz.R
import dk.cocode.parvaz.ui.theme.InkSoft

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun LanguageSection(
    currentLanguage: String,
    onLanguageChange: (String) -> Unit,
) {
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
