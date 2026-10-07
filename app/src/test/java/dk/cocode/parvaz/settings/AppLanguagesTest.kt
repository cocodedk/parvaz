package dk.cocode.parvaz.settings

import org.junit.Assert.assertEquals
import org.junit.Test

class AppLanguagesTest {
    @Test
    fun persianComesFirstAsTheDefaultAndDanishIsOffered() {
        assertEquals(listOf("fa", "en", "da"), AppLanguages.all)
    }

    @Test
    fun nextWalksThroughEveryLanguageAndWrapsRound() {
        assertEquals("en", AppLanguages.next("fa"))
        assertEquals("da", AppLanguages.next("en"))
        assertEquals("fa", AppLanguages.next("da"))
    }

    @Test
    fun nextOfAnUnknownCodeIsTheDefault() {
        assertEquals("fa", AppLanguages.next("de"))
    }
}
