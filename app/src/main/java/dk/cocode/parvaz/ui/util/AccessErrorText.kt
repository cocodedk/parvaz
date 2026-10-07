package dk.cocode.parvaz.ui.util

import androidx.annotation.StringRes
import dk.cocode.parvaz.R
import dk.cocode.parvaz.settings.AccessParseError

/** The text shown under the input field for each reason a parvaz:// link is refused. */
@StringRes
fun AccessParseError.textRes(): Int = when (this) {
    AccessParseError.NOT_PARVAZ_URL -> R.string.import_error_not_parvaz_url
    AccessParseError.NO_KEY -> R.string.import_error_no_key
    AccessParseError.EMPTY_ID -> R.string.import_error_empty_id
    AccessParseError.EMPTY_KEY -> R.string.import_error_empty_key
}
