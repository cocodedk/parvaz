package dk.cocode.parvaz

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import dk.cocode.parvaz.settings.Access
import dk.cocode.parvaz.settings.AccessImport
import dk.cocode.parvaz.settings.AccessParseException
import dk.cocode.parvaz.ui.theme.Ink
import dk.cocode.parvaz.ui.theme.InkSoft
import dk.cocode.parvaz.ui.theme.Oxblood
import dk.cocode.parvaz.ui.theme.Paper
import dk.cocode.parvaz.ui.theme.ParvazTheme

class MainActivity : ComponentActivity() {
    // Placeholder deep-link state — the real onboarding flow (M12)
    // replaces this with ImportAccessScreen + ViewModel-hoisted state.
    private var importedAccess by mutableStateOf<Access?>(null)
    private var importError by mutableStateOf<String?>(null)

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        handleDeepLink(intent)
        enableEdgeToEdge()
        setContent {
            ParvazTheme {
                Scaffold(
                    modifier = Modifier.fillMaxSize().background(Paper)
                ) { padding ->
                    SkeletonScreen(
                        imported = importedAccess,
                        error = importError,
                        modifier = Modifier.padding(padding),
                    )
                }
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        handleDeepLink(intent)
    }

    private fun handleDeepLink(intent: Intent?) {
        val uri = intent?.data?.toString() ?: return
        try {
            AccessImport.tryExtractFromUri(uri)?.let {
                importedAccess = it
                importError = null
            }
        } catch (e: AccessParseException) {
            importError = e.message
            importedAccess = null
        }
    }
}

@Composable
private fun SkeletonScreen(
    imported: Access?,
    error: String?,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier
            .fillMaxSize()
            .background(Paper)
            .padding(32.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(
            text = "§0 · SKELETON",
            style = MaterialTheme.typography.labelLarge,
            color = InkSoft,
        )
        Text(
            text = "Parvaz",
            style = MaterialTheme.typography.displayLarge,
            color = Ink,
        )
        when {
            imported != null -> {
                Text(
                    text = "Imported: ${imported.displayName ?: imported.deploymentId}",
                    style = MaterialTheme.typography.bodyMedium,
                    color = Ink,
                )
            }
            error != null -> {
                Text(
                    text = error,
                    style = MaterialTheme.typography.bodyMedium,
                    color = Oxblood,
                )
            }
            else -> {
                Text(
                    text = "CLEAR FOR FLIGHT — scaffolding only",
                    style = MaterialTheme.typography.labelSmall,
                    color = InkSoft,
                )
            }
        }
    }
}

@Preview(showBackground = true, backgroundColor = 0xFFF1E8D4)
@Composable
private fun SkeletonPreview() {
    ParvazTheme { SkeletonScreen(imported = null, error = null) }
}
