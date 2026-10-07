package dk.cocode.parvaz.settings

import java.net.URLDecoder
import java.net.URLEncoder

/**
 * Access is the sole piece of configuration a Parvaz user ever enters.
 * Parses from a `parvaz://<deployment-id>/<access-key>#<optional-display-name>`
 * URL — the same string a technical helper shares over Telegram or
 * encodes as a QR code.
 *
 * `<deployment-id>` is the `AKfycby...` segment a Google Apps Script
 * deployment URL carries; the full URL is derived on the fly.
 *
 * A broken string fails with an [AccessParseError]; the screen that shows it
 * looks the text up in the user's language.
 */
data class Access(
    val deploymentId: String,
    val accessKey: String,
    val displayName: String? = null,
) {
    /** Derived Apps Script deployment URL — what the Go sidecar POSTs envelopes to. */
    val deploymentURL: String
        get() = "https://script.google.com/macros/s/$deploymentId/exec"

    /** Canonical round-trip `parvaz://` string. */
    fun toURL(): String {
        val fragment = displayName?.takeIf(String::isNotBlank)?.let {
            "#" + URLEncoder.encode(it, "UTF-8").replace("+", "%20")
        } ?: ""
        return "parvaz://$deploymentId/$accessKey$fragment"
    }

    companion object {
        private const val SCHEME = "parvaz://"

        /**
         * Parse a user-supplied `parvaz://...` string. On failure, throws
         * [AccessParseException] carrying the [AccessParseError] to show.
         */
        fun parse(input: String): Access {
            val trimmed = input.trim()
            if (!trimmed.startsWith(SCHEME)) {
                throw AccessParseException(AccessParseError.NOT_PARVAZ_URL)
            }
            val withoutScheme = trimmed.removePrefix(SCHEME)

            val (pathPart, fragmentRaw) = withoutScheme.split("#", limit = 2).let {
                if (it.size == 2) it[0] to it[1] else it[0] to null
            }

            val slashIdx = pathPart.indexOf('/')
            if (slashIdx < 0) {
                throw AccessParseException(AccessParseError.NO_KEY)
            }
            val deploymentId = pathPart.substring(0, slashIdx).trim()
            val accessKey = pathPart.substring(slashIdx + 1).trim()

            if (deploymentId.isEmpty()) {
                throw AccessParseException(AccessParseError.EMPTY_ID)
            }
            if (accessKey.isEmpty()) {
                throw AccessParseException(AccessParseError.EMPTY_KEY)
            }

            val displayName = fragmentRaw
                ?.let { URLDecoder.decode(it, "UTF-8") }
                ?.takeIf(String::isNotBlank)

            return Access(
                deploymentId = deploymentId,
                accessKey = accessKey,
                displayName = displayName,
            )
        }
    }
}

/** Why a parvaz:// URL was refused. The text for each lives in strings.xml (`import_error_*`). */
enum class AccessParseError {
    NOT_PARVAZ_URL,
    NO_KEY,
    EMPTY_ID,
    EMPTY_KEY,
    ;

    companion object {
        /** For restoring a saved error by [name]; null for a missing or unknown name. */
        fun fromName(name: String?): AccessParseError? = entries.firstOrNull { it.name == name }
    }
}

/**
 * Thrown when a parvaz:// URL cannot be parsed. Catch this where the input
 * comes in (paste, QR, intent) and show the text for [error] under the
 * input field.
 */
class AccessParseException(val error: AccessParseError) : IllegalArgumentException(error.name)
