package dk.cocode.parvaz

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Scaffold
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import dk.cocode.parvaz.settings.Access
import dk.cocode.parvaz.settings.AccessImport
import dk.cocode.parvaz.settings.AccessParseException
import dk.cocode.parvaz.settings.ParvazSettings
import dk.cocode.parvaz.ui.onboarding.OnboardingHost
import dk.cocode.parvaz.ui.theme.Paper
import dk.cocode.parvaz.ui.theme.ParvazTheme

class MainActivity : ComponentActivity() {
    /**
     * Deep-link pre-fill for the ImportAccessScreen (M12.2). When set,
     * Import auto-populates the paste field. Placeholder state until
     * that screen lands.
     */
    private var pendingParvazUrl by mutableStateOf<String?>(null)
    private var pendingParvazUrlError by mutableStateOf<String?>(null)

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        handleDeepLink(intent)
        // Load once per activity creation; on rotation/process-death this
        // re-reads from disk so state survives without a bundle Saver.
        val bootstrapAccess: Access? = ParvazSettings(this).load()
        enableEdgeToEdge()
        setContent {
            ParvazTheme {
                Scaffold(
                    modifier = Modifier.fillMaxSize().background(Paper),
                ) { padding ->
                    OnboardingHost(
                        initialDeepLinkUrl = pendingParvazUrl,
                        initialDeepLinkError = pendingParvazUrlError,
                        alreadyImportedAccess = bootstrapAccess,
                        onFinished = {
                            // M13 (main screen) lands here. For now the
                            // flow just ends at DONE.
                        },
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
                pendingParvazUrl = uri
                pendingParvazUrlError = null
            }
        } catch (e: AccessParseException) {
            pendingParvazUrl = null
            pendingParvazUrlError = e.message
        }
    }
}
