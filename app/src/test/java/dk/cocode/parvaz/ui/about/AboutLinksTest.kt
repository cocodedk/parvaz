package dk.cocode.parvaz.ui.about

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Test

class AboutLinksTest {
    @Test
    fun updateButtonOpensTheGitHubReleaseUntilTheAppIsOnFDroid() {
        assertEquals(
            "https://github.com/cocodedk/parvaz/releases/latest",
            aboutUrl(AboutLink.Updates, "en", fdroidLive = false),
        )
    }

    @Test
    fun updateButtonOpensTheFDroidPageOnceTheAppIsThere() {
        for (language in listOf("fa", "en", "da")) {
            assertEquals(
                "https://f-droid.org/packages/dk.cocode.parvaz/",
                aboutUrl(AboutLink.Updates, language, fdroidLive = true),
            )
        }
    }

    @Test
    fun theAppShipsAsNotYetOnFDroid() {
        // apps.yml: fdroid is `mr:49559`, an open merge request. Flip FDROID_LIVE when it is accepted.
        assertFalse(FDROID_LIVE)
    }

    @Test
    fun privacyLinkIsLeftOutWhenThereIsNoPolicyAddress() {
        for (language in listOf("fa", "en", "da")) {
            assertNull(aboutUrl(AboutLink.Privacy, language, privacyUrl = null))
        }
    }

    @Test
    fun theAppShipsWithThePolicyAddressFromAppsYml() {
        assertEquals("https://parvaz.cocode.dk/privacy/", PRIVACY_URL)
        assertEquals("https://parvaz.cocode.dk/privacy/", aboutUrl(AboutLink.Privacy, "en"))
    }

    @Test
    fun privacyLinkOpensTheGivenPolicyAddressInEnglish() {
        val url = "https://parvaz.cocode.dk/privacy/"
        assertEquals(url, aboutUrl(AboutLink.Privacy, "en", privacyUrl = url))
    }

    @Test
    fun danishOpensTheDanishWebsiteAndPrivacyPages() {
        assertEquals("https://parvaz.cocode.dk/da/", aboutUrl(AboutLink.Website, "da"))
        assertEquals("https://parvaz.cocode.dk/da/privacy/", aboutUrl(AboutLink.Privacy, "da"))
    }

    @Test
    fun englishOpensTheEnglishWebsiteAndPrivacyPages() {
        assertEquals("https://parvaz.cocode.dk/", aboutUrl(AboutLink.Website, "en"))
        assertEquals("https://parvaz.cocode.dk/privacy/", aboutUrl(AboutLink.Privacy, "en"))
    }

    @Test
    fun aLanguageTheSiteLacksFallsBackToTheEnglishPages() {
        // Persian has /fa/ but no /fa/privacy/, so it stays on English; "fr" has no pages at all.
        for (language in listOf("fa", "fr")) {
            assertEquals("https://parvaz.cocode.dk/", aboutUrl(AboutLink.Website, language))
            assertEquals("https://parvaz.cocode.dk/privacy/", aboutUrl(AboutLink.Privacy, language))
        }
    }

    @Test
    fun anAddressOutsideTheSiteIsNeverRewritten() {
        val url = "https://example.org/privacy/"
        assertEquals(url, aboutUrl(AboutLink.Privacy, "da", privacyUrl = url))
    }

    @Test
    fun sourceAndIssuesIgnoreLanguage() {
        for (language in listOf("fa", "en", "da")) {
            assertEquals("https://github.com/cocodedk/parvaz", aboutUrl(AboutLink.Source, language))
            assertEquals("https://github.com/cocodedk/parvaz/issues", aboutUrl(AboutLink.Issues, language))
        }
    }
}
