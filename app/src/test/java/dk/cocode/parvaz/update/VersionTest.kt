package dk.cocode.parvaz.update

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class VersionTest {

    @Test fun parsesSemverWithVPrefix() {
        assertEquals(Version(0, 1, 4), Version.parse("v0.1.4"))
    }

    @Test fun parsesSemverWithoutVPrefix() {
        assertEquals(Version(1, 2, 3), Version.parse("1.2.3"))
    }

    @Test fun parsesTwoComponentAsZeroPatch() {
        assertEquals(Version(2, 5, 0), Version.parse("v2.5"))
    }

    @Test fun ignoresPrereleaseSuffix() {
        // GitHub release tags sometimes carry -rc.1 / -alpha — strip for compare.
        assertEquals(Version(0, 2, 0), Version.parse("v0.2.0-rc.1"))
    }

    @Test fun rejectsNonSemver() {
        assertNull(Version.parse(""))
        assertNull(Version.parse("nightly"))
        assertNull(Version.parse("v"))
    }

    @Test fun lessThanByMajorMinorPatch() {
        assertTrue(Version(0, 1, 4) < Version(0, 1, 5))
        assertTrue(Version(0, 1, 9) < Version(0, 2, 0))
        assertTrue(Version(0, 9, 9) < Version(1, 0, 0))
    }

    @Test fun equalsWhenAllMatch() {
        assertEquals(Version(0, 1, 4), Version(0, 1, 4))
    }

    @Test fun isNewerThanFalseWhenSame() {
        val v = Version(0, 1, 4)
        assertEquals(false, v.isNewerThan(v))
    }
}
