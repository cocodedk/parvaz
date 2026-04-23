package dk.cocode.parvaz.settings

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.fail
import org.junit.Test

class AccessTest {

    @Test
    fun parsesMinimalURL() {
        val a = Access.parse("parvaz://AKfycbyXYZ/SECRET")
        assertEquals("AKfycbyXYZ", a.deploymentId)
        assertEquals("SECRET", a.accessKey)
        assertNull(a.displayName)
        assertEquals(
            "https://script.google.com/macros/s/AKfycbyXYZ/exec",
            a.deploymentURL,
        )
    }

    @Test
    fun parsesDisplayNameWithSpaces() {
        val a = Access.parse("parvaz://id/key#My%20Relay")
        assertEquals("My Relay", a.displayName)
    }

    @Test
    fun parsesPersianDisplayName() {
        // "رله" URL-encoded as UTF-8
        val a = Access.parse("parvaz://id/key#%D8%B1%D9%84%D9%87")
        assertEquals("رله", a.displayName)
    }

    @Test
    fun trimsOuterWhitespace() {
        val a = Access.parse("  parvaz://id/key  ")
        assertEquals("id", a.deploymentId)
        assertEquals("key", a.accessKey)
    }

    @Test
    fun rejectsWrongScheme() {
        try {
            Access.parse("http://id/key")
            fail("expected AccessParseException")
        } catch (e: AccessParseException) {
            assertEquals("آدرس باید با parvaz:// شروع شود", e.message)
        }
    }

    @Test
    fun rejectsMissingKeyPath() {
        try {
            Access.parse("parvaz://AKfycbyXYZ")
            fail("expected AccessParseException")
        } catch (e: AccessParseException) {
            assertEquals("آدرس باید شامل کلید دسترسی باشد", e.message)
        }
    }

    @Test
    fun rejectsEmptyKey() {
        try {
            Access.parse("parvaz://id/")
            fail("expected AccessParseException")
        } catch (e: AccessParseException) {
            assertEquals("کلید دسترسی خالی است", e.message)
        }
    }

    @Test
    fun rejectsEmptyDeploymentId() {
        try {
            Access.parse("parvaz:///key")
            fail("expected AccessParseException")
        } catch (e: AccessParseException) {
            assertEquals("شناسهٔ دسترسی خالی است", e.message)
        }
    }

    @Test
    fun roundTripsWithPersianDisplayName() {
        val original = Access(
            deploymentId = "AKfycbyIRAN",
            accessKey = "KEY",
            displayName = "رلهٔ بابک",
        )
        val decoded = Access.parse(original.toURL())
        assertEquals(original, decoded)
    }

    @Test
    fun emptyDisplayNameBecomesNull() {
        val a = Access.parse("parvaz://id/key#")
        assertNull(a.displayName)
    }
}
