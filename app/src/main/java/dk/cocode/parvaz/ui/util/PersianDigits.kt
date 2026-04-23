package dk.cocode.parvaz.ui.util

/**
 * Map Latin digits 0-9 to Persian digits ۰-۹. Used for the main-screen
 * uptime display when the UI language is `fa`. Pure — no Android deps.
 */
fun String.toPersianDigits(): String {
    val sb = StringBuilder(length)
    for (c in this) {
        sb.append(
            when (c) {
                '0' -> '۰'
                '1' -> '۱'
                '2' -> '۲'
                '3' -> '۳'
                '4' -> '۴'
                '5' -> '۵'
                '6' -> '۶'
                '7' -> '۷'
                '8' -> '۸'
                '9' -> '۹'
                else -> c
            },
        )
    }
    return sb.toString()
}

/**
 * Format uptime seconds as `T+HH:MM:SS`. Locale-aware: returns Persian
 * digits when [persian] is true, Latin otherwise. Negative input clamps
 * to zero.
 */
fun formatUptime(seconds: Long, persian: Boolean): String {
    val s = if (seconds < 0) 0 else seconds
    val hh = s / 3600
    val mm = (s % 3600) / 60
    val ss = s % 60
    val latin = "T+%02d:%02d:%02d".format(hh, mm, ss)
    return if (persian) latin.toPersianDigits() else latin
}
