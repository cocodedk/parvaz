package dk.cocode.parvaz

import android.content.Context
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertTextContains
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextInput
import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import dk.cocode.parvaz.settings.ParvazSettings
import dk.cocode.parvaz.ui.about.AboutTestTags
import dk.cocode.parvaz.ui.onboarding.TestTags
import dk.cocode.parvaz.ui.settings.SettingsTestTags
import java.io.File
import org.junit.After
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

/**
 * Opening the About page and closing it must leave setup where it was:
 * on a fresh install, a link pasted into the Import step is still there,
 * and the screen is still the Import step, not the Splash step.
 */
@RunWith(AndroidJUnit4::class)
class AboutKeepsSetupStateTest {

    private val context: Context = ApplicationProvider.getApplicationContext()

    @get:Rule
    val composeRule = createAndroidComposeRule<MainActivity>()

    @Before
    fun startFresh() {
        wipeState()
        composeRule.activityRule.scenario.recreate()
    }

    @After
    fun tearDown() {
        wipeState()
    }

    @Test
    fun linkTypedInImportSurvivesOpeningAndClosingAbout() {
        composeRule.onNodeWithTag(TestTags.SplashStartButton).performClick()
        composeRule.onNodeWithTag(TestTags.ImportField).performTextInput(LINK)

        composeRule.onNodeWithTag(SettingsTestTags.Gear).performClick()
        composeRule.onNodeWithTag(SettingsTestTags.AboutButton).performClick()
        composeRule.onNodeWithTag(AboutTestTags.Screen).assertIsDisplayed()

        composeRule.onNodeWithTag(AboutTestTags.Close).performClick()

        composeRule.onNodeWithTag(TestTags.ImportField).assertTextContains(LINK)
    }

    private fun wipeState() {
        context.deleteSharedPreferences(PLAIN_FILE)
        context.deleteSharedPreferences(SECURE_FILE)
        File(context.filesDir, "parvaz-data").deleteRecursively()
    }

    private companion object {
        const val PLAIN_FILE = "parvaz_prefs"
        const val SECURE_FILE = "parvaz_secure"
        const val LINK = "parvaz://AKfycby_about_dddddddddddddddddddd/about-key"
    }
}
