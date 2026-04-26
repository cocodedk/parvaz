package dk.cocode.parvaz

import android.content.Context
import android.content.Intent
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.waitUntilExactlyOneExists
import androidx.core.net.toUri
import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import dk.cocode.parvaz.settings.Access
import dk.cocode.parvaz.settings.ParvazSettings
import dk.cocode.parvaz.ui.onboarding.TestTags
import org.junit.After
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

/**
 * Regression test for the deep-link recreate bug.
 *
 * Bug: when a user with saved access taps a fresh `parvaz://` link,
 * MainActivity routes to the IMPORT step. Tapping the onboarding
 * language toggle calls `recreate()`, which historically destroyed
 * `pendingParvazUrl` (an Activity field, reset to null on rebuild).
 * After recreate, the user could be bounced away from IMPORT before
 * the new access was imported.
 *
 * Fix: pendingParvazUrl/Error are now persisted in
 * MainActivity.onSaveInstanceState and restored in onCreate.
 *
 * The test launches MainActivity with a default (LAUNCHER) intent —
 * launching directly with a parvaz:// data Intent under ActivityScenario
 * hits a splash-screen / Compose-test deadlock. The deep link is
 * injected after the activity settles via the @VisibleForTesting
 * `handleDeepLink` entry point.
 */
@OptIn(ExperimentalTestApi::class)
@RunWith(AndroidJUnit4::class)
class MainActivityDeepLinkRecreateTest {

    private val context: Context = ApplicationProvider.getApplicationContext()

    @get:Rule
    val composeRule = createAndroidComposeRule<MainActivity>()

    @Before
    fun seedOnboardedUser() {
        // Seed AFTER the rule has launched, then recreate so the
        // activity picks up disk state. The CA/VPN readiness gate may
        // send stale completion back to onboarding, but the deep-link
        // payload must still win and keep IMPORT visible.
        context.deleteSharedPreferences(PLAIN_FILE)
        context.deleteSharedPreferences(SECURE_FILE)
        val s = ParvazSettings(context)
        s.save(Access(EXISTING_DEPLOYMENT, EXISTING_KEY, "regression"))
        s.isOnboardingComplete = true
        composeRule.activityRule.scenario.recreate()
        composeRule.waitForIdle()
    }

    @After
    fun wipe() {
        context.deleteSharedPreferences(PLAIN_FILE)
        context.deleteSharedPreferences(SECURE_FILE)
    }

    @Test
    fun deepLinkImportSurvivesActivityRecreate() {
        // Inject the deep link the same way onNewIntent would, then let
        // recomposition catch up.
        val deepLink = Intent().apply {
            data = "parvaz://$NEW_DEPLOYMENT/$NEW_KEY#new-relay".toUri()
        }
        composeRule.activityRule.scenario.onActivity { it.handleDeepLink(deepLink) }
        composeRule.waitUntilExactlyOneExists(hasTestTag(TestTags.ImportField), 5_000)
        composeRule.onNodeWithTag(TestTags.ImportField).assertIsDisplayed()

        // The actual regression check: trigger the same code path the
        // onboarding language toggle exercises.
        composeRule.activityRule.scenario.recreate()

        // Without the fix, pendingParvazUrl is null after recreate and
        // IMPORT is gone.
        composeRule.waitUntilExactlyOneExists(hasTestTag(TestTags.ImportField), 5_000)
        composeRule.onNodeWithTag(TestTags.ImportField).assertIsDisplayed()
    }

    private companion object {
        const val PLAIN_FILE = "parvaz_prefs"
        const val SECURE_FILE = "parvaz_secure"
        const val EXISTING_DEPLOYMENT = "AKfycby_existing_aaaaaaaaaaaaaaaaaaaaaaaaaaa"
        const val EXISTING_KEY = "existing-key-32-chars-aaaaaaaaaaaaaaa"
        const val NEW_DEPLOYMENT = "AKfycby_new_bbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
        const val NEW_KEY = "new-incoming-key-32-chars-bbbbbbbbbbbbb"
    }
}
