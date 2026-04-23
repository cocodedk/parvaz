package dk.cocode.parvaz.ui.util

import org.junit.Assert.assertEquals
import org.junit.Test

class PersianDigitsTest {

    @Test
    fun allDigitsMapped() {
        assertEquals("۰۱۲۳۴۵۶۷۸۹", "0123456789".toPersianDigits())
    }

    @Test
    fun nonDigitsPassThrough() {
        assertEquals("T+۰۰:۱۲:۴۷", "T+00:12:47".toPersianDigits())
        assertEquals("abc۷۸۹", "abc789".toPersianDigits())
    }

    @Test
    fun emptyStayEmpty() {
        assertEquals("", "".toPersianDigits())
    }

    @Test
    fun formatUptime_zeroSeconds() {
        assertEquals("T+00:00:00", formatUptime(0, persian = false))
        assertEquals("T+۰۰:۰۰:۰۰", formatUptime(0, persian = true))
    }

    @Test
    fun formatUptime_mixed() {
        // 12 min 47 sec → 767 seconds
        assertEquals("T+00:12:47", formatUptime(767, persian = false))
        assertEquals("T+۰۰:۱۲:۴۷", formatUptime(767, persian = true))
    }

    @Test
    fun formatUptime_overAnHour() {
        // 3661 = 1h 1m 1s
        assertEquals("T+01:01:01", formatUptime(3661, persian = false))
    }

    @Test
    fun formatUptime_negativeClampsToZero() {
        assertEquals("T+00:00:00", formatUptime(-5, persian = false))
    }
}
