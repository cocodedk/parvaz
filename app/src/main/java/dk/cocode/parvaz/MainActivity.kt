package dk.cocode.parvaz

import android.content.Context
import android.content.Intent
import android.content.res.Configuration
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.viewModels
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.core.splashscreen.SplashScreen.Companion.installSplashScreen
import androidx.lifecycle.lifecycleScope
import dk.cocode.parvaz.onboarding.isOnboardingStillReady
import dk.cocode.parvaz.settings.Access
import dk.cocode.parvaz.settings.AccessImport
import dk.cocode.parvaz.settings.AccessParseException
import dk.cocode.parvaz.settings.ParvazSettings
import dk.cocode.parvaz.ui.main.AppRoot
import dk.cocode.parvaz.ui.main.MainViewModel
import dk.cocode.parvaz.ui.theme.ParvazTheme
import dk.cocode.parvaz.update.UpdateController
import kotlinx.coroutines.launch
import java.util.Locale

private const val KEY_PENDING_URL = "pending_parvaz_url"
private const val KEY_PENDING_URL_ERROR = "pending_parvaz_url_error"

class MainActivity : ComponentActivity() {
    private val mainViewModel: MainViewModel by viewModels()
    private val updateController: UpdateController by viewModels()

    private var pendingParvazUrl by mutableStateOf<String?>(null)
    private var pendingParvazUrlError by mutableStateOf<String?>(null)

    private var activeAccess by mutableStateOf<Access?>(null)
    private var onboardingComplete by mutableStateOf(false)
    private var onboardingReadinessChecked by mutableStateOf(true)
    private var showSettingsSheet by mutableStateOf(false)

    /**
     * Override the base Context's locale with ParvazSettings.language
     * (default `fa`). Activity recreate() triggers a fresh
     * attachBaseContext, which is how the settings sheet's language
     * toggle takes effect without requiring a process restart.
     */
    override fun attachBaseContext(newBase: Context) {
        val lang = ParvazSettings(newBase).language
        val locale = Locale.forLanguageTag(lang)
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
        // recreate() (e.g. language toggle) destroys the activity; the
        // intent's data was already consumed by the original onCreate so
        // handleDeepLink will no-op on rebuild. Restore the parsed deep
        // link from saved state instead — otherwise the user gets bounced
        // out of the IMPORT step and into MainScreen on every recreate.
        savedInstanceState?.let {
            pendingParvazUrl = it.getString(KEY_PENDING_URL)
            pendingParvazUrlError = it.getString(KEY_PENDING_URL_ERROR)
        }
        handleDeepLink(intent)
        val settings = ParvazSettings(this)
        activeAccess = settings.load()
        val storedOnboardingComplete = settings.isOnboardingComplete
        onboardingReadinessChecked = !storedOnboardingComplete
        onboardingComplete = false
        if (storedOnboardingComplete) revalidateOnboarding(settings)
        enableEdgeToEdge()
        setContent {
            ParvazTheme {
                AppRoot(
                    mainViewModel = mainViewModel,
                    updateController = updateController,
                    pendingParvazUrl = pendingParvazUrl,
                    pendingParvazUrlError = pendingParvazUrlError,
                    activeAccess = activeAccess,
                    onboardingComplete = onboardingComplete,
                    onboardingReadinessChecked = onboardingReadinessChecked,
                    showSettingsSheet = showSettingsSheet,
                    onSettingsVisibilityChange = { showSettingsSheet = it },
                    currentLanguage = ParvazSettings(this).language,
                    onLanguageChange = { newLang ->
                        ParvazSettings(this).language = newLang
                        recreate()
                    },
                    onSaveAccess = { newAccess ->
                        ParvazSettings(this).save(newAccess)
                        activeAccess = newAccess
                    },
                    onResetAccess = {
                        val s = ParvazSettings(this)
                        s.clearAccess()
                        s.isOnboardingComplete = false
                        mainViewModel.disconnect()
                        activeAccess = null
                        onboardingComplete = false
                    },
                    onOnboardingFinished = { finished ->
                        ParvazSettings(this).isOnboardingComplete = true
                        onboardingComplete = true
                        activeAccess = finished
                        pendingParvazUrl = null
                        pendingParvazUrlError = null
                    },
                )
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        handleDeepLink(intent)
    }

    // internal (not private) so MainActivityDeepLinkRecreateTest can drive
    // the deep-link path without launching the activity with an intent —
    // launching with a parvaz:// intent under ActivityScenario hits a
    // splash-screen / Compose-test deadlock (PRE_ON_CREATE timeout).
    internal fun handleDeepLink(intent: Intent?) {
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

    private fun revalidateOnboarding(settings: ParvazSettings) {
        lifecycleScope.launch {
            val ready = isOnboardingStillReady(this@MainActivity, activeAccess)
            if (!ready) settings.isOnboardingComplete = false
            onboardingComplete = ready
            onboardingReadinessChecked = true
        }
    }

    override fun onSaveInstanceState(outState: Bundle) {
        super.onSaveInstanceState(outState)
        pendingParvazUrl?.let { outState.putString(KEY_PENDING_URL, it) }
        pendingParvazUrlError?.let { outState.putString(KEY_PENDING_URL_ERROR, it) }
    }
}
