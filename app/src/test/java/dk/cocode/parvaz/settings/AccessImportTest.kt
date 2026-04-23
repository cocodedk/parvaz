package dk.cocode.parvaz.settings

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertThrows
import org.junit.Test

/**
 * Unit tests for AccessImport — the pure string-level extractor used by
 * MainActivity to turn an android.net.Uri from an intent into an Access.
 * Keeping this function Uri-free lets us test it without instrumentation.
 */
class AccessImportTest {
    @Test
    fun `null input returns null`() {
        assertNull(AccessImport.tryExtractFromUri(null))
    }

    @Test
    fun `blank input returns null`() {
        assertNull(AccessImport.tryExtractFromUri(""))
        assertNull(AccessImport.tryExtractFromUri("   "))
    }

    @Test
    fun `wrong scheme returns null — not our deep link to handle`() {
        assertNull(AccessImport.tryExtractFromUri("https://example.com/foo"))
        assertNull(AccessImport.tryExtractFromUri("somethingelse://abc/def"))
    }

    @Test
    fun `valid parvaz url returns parsed Access`() {
        val got = AccessImport.tryExtractFromUri("parvaz://DEP123/KEY456#My%20Phone")
        assertEquals("DEP123", got?.deploymentId)
        assertEquals("KEY456", got?.accessKey)
        assertEquals("My Phone", got?.displayName)
    }

    @Test
    fun `parvaz url missing key rethrows AccessParseException`() {
        // The extractor deliberately does not swallow parse errors — the
        // caller (MainActivity) shows the Farsi message to the user.
        assertThrows(AccessParseException::class.java) {
            AccessImport.tryExtractFromUri("parvaz://DEP123")
        }
    }
}
