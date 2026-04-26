package dk.cocode.parvaz

import android.content.Context
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import dk.cocode.parvaz.settings.Access
import dk.cocode.parvaz.settings.ParvazSettings
import java.io.File
import org.junit.After
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

/**
 * Regression coverage for stale onboarding state: a saved completion
 * flag alone must not route to MainScreen. The device must still have
 * the matching Parvaz CA installed and VPN permission granted.
 */
@RunWith(AndroidJUnit4::class)
class MainActivityStaleOnboardingTest {

    private val context: Context = ApplicationProvider.getApplicationContext()

    @get:Rule
    val composeRule = createAndroidComposeRule<MainActivity>()

    @Before
    fun seedStaleOnboardingState() {
        wipeState()
        ParvazSettings(context).apply {
            save(Access(DEPLOYMENT, ACCESS_KEY, "stale-readiness"))
            isOnboardingComplete = true
        }
        composeRule.activityRule.scenario.recreate()
    }

    @After
    fun tearDown() {
        wipeState()
    }

    @Test
    fun missingCaClearsStoredCompletion() {
        composeRule.waitUntil(5_000) {
            !ParvazSettings(context).isOnboardingComplete
        }
    }

    private fun wipeState() {
        context.deleteSharedPreferences(PLAIN_FILE)
        context.deleteSharedPreferences(SECURE_FILE)
        File(context.filesDir, "parvaz-data").deleteRecursively()
    }

    private companion object {
        const val PLAIN_FILE = "parvaz_prefs"
        const val SECURE_FILE = "parvaz_secure"
        const val DEPLOYMENT = "AKfycby_stale_ccccccccccccccccccccccccccc"
        const val ACCESS_KEY = "stale-key-32-chars-ccccccccccccccc"
    }
}
