package dk.cocode.parvaz.settings

/**
 * AccessImport extracts a parvaz://-scheme URL from whatever opaque
 * carrier it arrived in (an Android Intent, a clipboard string, a QR
 * payload) into an [Access].
 *
 * The function deliberately takes a String rather than android.net.Uri
 * so it can be unit-tested on the JVM without instrumentation. Callers
 * pass `intent.data?.toString()` (or equivalent).
 *
 * Returns null when the input is not a parvaz:// URL — that's the
 * "nothing to do" case, not an error. Invalid parvaz:// URLs throw
 * [AccessParseException] so the caller can surface the Farsi message
 * directly under the input field.
 */
object AccessImport {
    private const val SCHEME_PREFIX = "parvaz://"

    fun tryExtractFromUri(uriString: String?): Access? {
        val trimmed = uriString?.trim().orEmpty()
        if (trimmed.isEmpty() || !trimmed.startsWith(SCHEME_PREFIX)) {
            return null
        }
        return Access.parse(trimmed)
    }
}
