package dk.cocode.parvaz

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.viewModels
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Scaffold
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalConfiguration
import dk.cocode.parvaz.settings.Access
import dk.cocode.parvaz.settings.AccessImport
import dk.cocode.parvaz.settings.AccessParseException
import dk.cocode.parvaz.settings.ParvazSettings
import dk.cocode.parvaz.ui.main.MainScreen
import dk.cocode.parvaz.ui.main.MainViewModel
import dk.cocode.parvaz.ui.onboarding.OnboardingHost
import dk.cocode.parvaz.ui.theme.Paper
import dk.cocode.parvaz.ui.theme.ParvazTheme

class MainActivity : ComponentActivity() {
    private val mainViewModel: MainViewModel by viewModels()

    /**
     * Deep-link pre-fill for the ImportAccessScreen (M12.2). When set,
     * Import auto-populates the paste field. Placeholder state until
     * that screen lands.
     */
    private var pendingParvazUrl by mutableStateOf<String?>(null)
    private var pendingParvazUrlError by mutableStateOf<String?>(null)

    /**
     * Activity-level record of which `Access` has been imported and
     * whether onboarding is fully done. Seeded from disk on create,
     * updated when onboarding finishes. Both conditions are required
     * to flip to MainScreen — a persisted Access alone is not enough;
     * M12.2 saves Access before CA install + VPN consent.
     */
    private var activeAccess by mutableStateOf<Access?>(null)
    private var onboardingComplete by mutableStateOf(false)

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        handleDeepLink(intent)
        val settings = ParvazSettings(this)
        activeAccess = settings.load()
        onboardingComplete = settings.isOnboardingComplete
        enableEdgeToEdge()
        setContent {
            ParvazTheme {
                Scaffold(modifier = Modifier.fillMaxSize().background(Paper)) { padding ->
                    val access = activeAccess
                    val hasDeepLink = pendingParvazUrl != null || pendingParvazUrlError != null
                    val showMain = access != null && onboardingComplete && !hasDeepLink
                    if (showMain) {
                        val persianDigits = LocalConfiguration.current.locales.get(0)?.language == "fa"
                        MainScreen(
                            viewModel = mainViewModel,
                            persianNumerals = persianDigits,
                            modifier = Modifier.padding(padding),
                        )
                    } else {
                        OnboardingHost(
                            initialDeepLinkUrl = pendingParvazUrl,
                            initialDeepLinkError = pendingParvazUrlError,
                            alreadyImportedAccess = access,
                            onFinished = { finished ->
                                ParvazSettings(this@MainActivity).isOnboardingComplete = true
                                onboardingComplete = true
                                activeAccess = finished
                                pendingParvazUrl = null
                                pendingParvazUrlError = null
                            },
                            modifier = Modifier.padding(padding),
                        )
                    }
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
