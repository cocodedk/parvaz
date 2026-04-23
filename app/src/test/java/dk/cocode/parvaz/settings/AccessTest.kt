package dk.cocode.parvaz.settings

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.fail
import org.junit.Test

class AccessTest {

    @Test
    fun parsesMinimalURL() {
        val a = Access.parse("parvaz://relay.workers.dev/ABC123")
        assertEquals("relay.workers.dev", a.host)
        assertEquals("ABC123", a.accessKey)
        assertNull(a.displayName)
        assertEquals("wss://relay.workers.dev/tunnel", a.workerURL)
    }

    @Test
    fun parsesDisplayNameWithSpaces() {
        val a = Access.parse("parvaz://h/k#My%20Relay")
        assertEquals("My Relay", a.displayName)
    }

    @Test
    fun parsesPersianDisplayName() {
        // "رله" URL-encoded as UTF-8
        val a = Access.parse("parvaz://h/k#%D8%B1%D9%84%D9%87")
        assertEquals("رله", a.displayName)
    }

    @Test
    fun trimsOuterWhitespace() {
        val a = Access.parse("  parvaz://relay/key  ")
        assertEquals("relay", a.host)
        assertEquals("key", a.accessKey)
    }

    @Test
    fun rejectsWrongScheme() {
        try {
            Access.parse("http://relay.workers.dev/key")
            fail("expected AccessParseException")
        } catch (e: AccessParseException) {
            assertEquals("آدرس باید با parvaz:// شروع شود", e.message)
        }
    }

    @Test
    fun rejectsMissingKeyPath() {
        try {
            Access.parse("parvaz://relay.workers.dev")
            fail("expected AccessParseException")
        } catch (e: AccessParseException) {
            assertEquals("آدرس باید شامل کلید دسترسی باشد", e.message)
        }
    }

    @Test
    fun rejectsEmptyKey() {
        try {
            Access.parse("parvaz://relay.workers.dev/")
            fail("expected AccessParseException")
        } catch (e: AccessParseException) {
            assertEquals("کلید دسترسی خالی است", e.message)
        }
    }

    @Test
    fun rejectsEmptyHost() {
        try {
            Access.parse("parvaz:///key")
            fail("expected AccessParseException")
        } catch (e: AccessParseException) {
            assertEquals("آدرس سرور خالی است", e.message)
        }
    }

    @Test
    fun roundTripsWithPersianDisplayName() {
        val original = Access(
            host = "relay-iran.workers.dev",
            accessKey = "KEY",
            displayName = "رلهٔ بابک",
        )
        val decoded = Access.parse(original.toURL())
        assertEquals(original, decoded)
    }

    @Test
    fun emptyDisplayNameBecomesNull() {
        val a = Access.parse("parvaz://h/k#")
        assertNull(a.displayName)
    }
}
