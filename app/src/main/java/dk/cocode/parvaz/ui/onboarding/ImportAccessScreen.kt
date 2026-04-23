package dk.cocode.parvaz.ui.onboarding

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextFieldDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import dk.cocode.parvaz.R
import dk.cocode.parvaz.settings.Access
import dk.cocode.parvaz.settings.AccessParseException
import dk.cocode.parvaz.settings.ParvazSettings
import dk.cocode.parvaz.ui.theme.Ink
import dk.cocode.parvaz.ui.theme.InkSoft
import dk.cocode.parvaz.ui.theme.Oxblood
import dk.cocode.parvaz.ui.theme.Paper
import dk.cocode.parvaz.ui.theme.ParvazTheme

/**
 * Step 2 of onboarding. One text field for the parvaz:// URL. Paste
 * pulls the clipboard; Continue validates via [Access.parse] and
 * persists through [ParvazSettings] before handing off to [onImported].
 * QR scanning arrives as M14-completion on top of this screen.
 */
@Composable
fun ImportAccessScreen(
    initialUrl: String?,
    initialError: String?,
    onImported: (Access) -> Unit,
    modifier: Modifier = Modifier,
) {
    var text by rememberSaveable { mutableStateOf(initialUrl.orEmpty()) }
    var error by rememberSaveable { mutableStateOf(initialError) }

    // A fresh parvaz:// intent delivered via onNewIntent updates our
    // callers' initialUrl/initialError while this screen is already on
    // screen. rememberSaveable keys off first composition only, so we
    // re-seed the field when those inputs change.
    LaunchedEffect(initialUrl, initialError) {
        if (initialUrl != null) {
            text = initialUrl
            error = initialError
        } else if (initialError != null) {
            error = initialError
        }
    }

    val clipboard = LocalClipboardManager.current
    val context = LocalContext.current

    Column(
        modifier = modifier
            .fillMaxSize()
            .background(Paper)
            .padding(horizontal = 32.dp, vertical = 48.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Text(
            text = stringResource(R.string.import_title),
            style = MaterialTheme.typography.displayMedium,
            color = Ink,
        )
        Text(
            text = stringResource(R.string.import_subtitle),
            style = MaterialTheme.typography.bodyMedium,
            color = InkSoft,
        )
        Spacer(Modifier.height(8.dp))

        OutlinedTextField(
            value = text,
            onValueChange = {
                text = it
                error = null
            },
            placeholder = { Text(stringResource(R.string.import_field_hint)) },
            singleLine = false,
            isError = error != null,
            keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.None),
            colors = TextFieldDefaults.colors(
                focusedContainerColor = Paper,
                unfocusedContainerColor = Paper,
                focusedIndicatorColor = Ink,
                unfocusedIndicatorColor = InkSoft,
                errorIndicatorColor = Oxblood,
            ),
            modifier = Modifier
                .fillMaxWidth()
                .testTag(TestTags.ImportField),
        )
        if (error != null) {
            Text(
                text = error!!,
                style = MaterialTheme.typography.bodyMedium,
                color = Oxblood,
                modifier = Modifier.testTag(TestTags.ImportErrorText),
            )
        }

        Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            OutlinedButton(
                onClick = {
                    clipboard.getText()?.text?.let {
                        text = it
                        error = null
                    }
                },
                modifier = Modifier.testTag(TestTags.ImportPasteButton),
            ) {
                Text(stringResource(R.string.import_paste_cta))
            }
            OutlinedButton(
                onClick = { /* TODO M14-QR */ },
                enabled = false,
                modifier = Modifier.testTag(TestTags.ImportScanButton),
            ) {
                Text(stringResource(R.string.import_scan_cta))
            }
        }

        Spacer(Modifier.height(8.dp))

        Button(
            onClick = {
                try {
                    val access = Access.parse(text)
                    ParvazSettings(context).save(access)
                    onImported(access)
                } catch (e: AccessParseException) {
                    error = e.message ?: "parvaz://?"
                }
            },
            enabled = text.isNotBlank(),
            colors = ButtonDefaults.buttonColors(
                containerColor = Oxblood,
                contentColor = Paper,
            ),
            shape = RoundedCornerShape(2.dp),
            modifier = Modifier
                .fillMaxWidth()
                .testTag(TestTags.ImportSubmitButton),
        ) {
            Text(
                text = stringResource(R.string.import_submit_cta),
                style = MaterialTheme.typography.headlineMedium,
            )
        }
    }
}

@Preview(showBackground = true, backgroundColor = 0xFFF1E8D4)
@Composable
private fun ImportAccessScreenPreview() {
    ParvazTheme {
        ImportAccessScreen(
            initialUrl = "parvaz://DEP/KEY#Phone",
            initialError = null,
            onImported = {},
        )
    }
}
