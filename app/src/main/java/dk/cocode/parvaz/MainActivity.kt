package dk.cocode.parvaz

import android.content.Context
import android.content.Intent
import android.content.res.Configuration
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.viewModels
import androidx.core.splashscreen.SplashScreen.Companion.installSplashScreen
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Scaffold
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.testTagsAsResourceId
import dk.cocode.parvaz.settings.Access
import dk.cocode.parvaz.settings.AccessImport
import dk.cocode.parvaz.settings.AccessParseException
import dk.cocode.parvaz.settings.ParvazSettings
import dk.cocode.parvaz.ui.main.MainScreen
import dk.cocode.parvaz.ui.main.MainSettingsSheet
import dk.cocode.parvaz.ui.main.MainViewModel
import dk.cocode.parvaz.ui.onboarding.OnboardingHost
import dk.cocode.parvaz.ui.theme.Paper
import dk.cocode.parvaz.ui.theme.ParvazTheme
import java.util.Locale

class MainActivity : ComponentActivity() {
    private val mainViewModel: MainViewModel by viewModels()

    private var pendingParvazUrl by mutableStateOf<String?>(null)
    private var pendingParvazUrlError by mutableStateOf<String?>(null)

    private var activeAccess by mutableStateOf<Access?>(null)
    private var onboardingComplete by mutableStateOf(false)
    private var showSettingsSheet by mutableStateOf(false)

    /**
     * Override the base Context's locale with ParvazSettings.language
     * (default `fa`). Activity recreate() triggers a fresh
     * attachBaseContext, which is how the hidden settings sheet's
     * language toggle takes effect without requiring a process restart.
     */
    override fun attachBaseContext(newBase: Context) {
        val lang = ParvazSettings(newBase).language
        val locale = Locale(lang)
        Locale.setDefault(locale)
        val config = Configuration(newBase.resources.configuration)
        config.setLocale(locale)
        super.attachBaseContext(newBase.createConfigurationContext(config))
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        // Must run before super.onCreate so the SplashScreen API can swap
        // Theme.Parvaz.Starting (the launch theme) for Theme.Parvaz (the
        // post-splash app theme) in one frame, with no flash.
        installSplashScreen()
        super.onCreate(savedInstanceState)
        handleDeepLink(intent)
        val settings = ParvazSettings(this)
        activeAccess = settings.load()
        onboardingComplete = settings.isOnboardingComplete
        enableEdgeToEdge()
        setContent {
            ParvazTheme {
                // testTagsAsResourceId exposes Compose testTag values to
                // `uiautomator dump` as resource-id, so the e2e shell
                // script under scripts/e2e/ can drive the onboarding
                // flow without resorting to hardcoded coordinates.
                Scaffold(
                    modifier = Modifier
                        .fillMaxSize()
                        .background(Paper)
                        .semantics { testTagsAsResourceId = true },
                ) { padding ->
                    val access = activeAccess
                    val hasDeepLink = pendingParvazUrl != null || pendingParvazUrlError != null
                    val showMain = access != null && onboardingComplete && !hasDeepLink
                    if (showMain) {
                        val persianDigits = LocalConfiguration.current.locales.get(0)?.language == "fa"
                        MainScreen(
                            viewModel = mainViewModel,
                            persianNumerals = persianDigits,
                            onOpenSettings = { showSettingsSheet = true },
                            modifier = Modifier.padding(padding),
                        )
                        if (showSettingsSheet) {
                            MainSettingsSheet(
                                currentLanguage = ParvazSettings(this@MainActivity).language,
                                onLanguageChange = { newLang ->
                                    ParvazSettings(this@MainActivity).language = newLang
                                    showSettingsSheet = false
                                    recreate()
                                },
                                onResetAccess = {
                                    val s = ParvazSettings(this@MainActivity)
                                    s.clearAccess()
                                    s.isOnboardingComplete = false
                                    mainViewModel.disconnect()
                                    showSettingsSheet = false
                                    activeAccess = null
                                    onboardingComplete = false
                                },
                                onDismiss = { showSettingsSheet = false },
                            )
                        }
                    } else {
                        OnboardingHost(
                            initialDeepLinkUrl = pendingParvazUrl,
                            initialDeepLinkError = pendingParvazUrlError,
                            alreadyImportedAccess = access,
                            onLanguageChange = { newLang ->
                                ParvazSettings(this@MainActivity).language = newLang
                                recreate()
                            },
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
        // Consume the URI so a later recreate() (e.g. language toggle)
        // doesn't replay it and kick the user back into IMPORT.
        intent.data = null
    }
}
