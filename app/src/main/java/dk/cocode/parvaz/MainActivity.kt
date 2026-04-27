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
import androidx.lifecycle.lifecycleScope
import dk.cocode.parvaz.onboarding.isOnboardingStillReady
import dk.cocode.parvaz.settings.Access
import dk.cocode.parvaz.settings.AccessImport
import dk.cocode.parvaz.settings.AccessParseException
import dk.cocode.parvaz.settings.ParvazSettings
import dk.cocode.parvaz.ui.main.MainScreen
import dk.cocode.parvaz.ui.main.MainViewModel
import dk.cocode.parvaz.ui.onboarding.OnboardingHost
import dk.cocode.parvaz.ui.onboarding.ReadinessScreen
import dk.cocode.parvaz.ui.settings.SettingsScaffold
import dk.cocode.parvaz.ui.settings.SettingsSheet
import dk.cocode.parvaz.ui.settings.UpdateSection
import dk.cocode.parvaz.update.UpdateController
import dk.cocode.parvaz.update.Version
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.compose.runtime.getValue as composeGetValue
import dk.cocode.parvaz.ui.theme.Paper
import dk.cocode.parvaz.ui.theme.ParvazTheme
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
     * attachBaseContext, which is how the hidden settings sheet's
     * language toggle takes effect without requiring a process restart.
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
                    val checkingReadiness = access != null && !onboardingReadinessChecked && !hasDeepLink
                    SettingsScaffold(
                        onOpenSettings = { showSettingsSheet = true },
                        modifier = Modifier.padding(padding),
                    ) {
                        if (showMain) {
                            val persianDigits = LocalConfiguration.current.locales.get(0)?.language == "fa"
                            MainScreen(
                                viewModel = mainViewModel,
                                persianNumerals = persianDigits,
                                onOpenSettings = { showSettingsSheet = true },
                            )
                        } else if (checkingReadiness) {
                            ReadinessScreen()
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
                            )
                        }
                    }
                    if (showSettingsSheet) {
                        val updateState by updateController.state.collectAsStateWithLifecycle()
                        SettingsSheet(
                            currentLanguage = ParvazSettings(this@MainActivity).language,
                            currentAccess = activeAccess,
                            onboardingComplete = onboardingComplete,
                            onLanguageChange = { newLang ->
                                ParvazSettings(this@MainActivity).language = newLang
                                showSettingsSheet = false
                                recreate()
                            },
                            onSaveAccess = { newAccess ->
                                ParvazSettings(this@MainActivity).save(newAccess)
                                activeAccess = newAccess
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
                            updateSection = {
                                UpdateSection(
                                    currentVersionName = BuildConfig.VERSION_NAME,
                                    state = updateState,
                                    onCheck = {
                                        val current = Version.parse(BuildConfig.VERSION_NAME)
                                            ?: Version(0, 0, 0)
                                        updateController.check(current)
                                    },
                                    onInstall = { updateController.install(stopVpn = { mainViewModel.disconnect() }) },
                                )
                            },
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
