package dk.cocode.parvaz.settings

import android.content.Context
import java.io.File

/**
 * Single source of truth for the sidecar's persistent data directory.
 * Lives inside `context.filesDir` so Android owns the lifecycle — it's
 * private to the app, survives updates, and gets cleaned on uninstall.
 *
 * The Go sidecar reads + writes under `<dataDir>/ca/` for the MITM CA.
 * Everything (parvazd start-up, `-gen-ca`, CA verification) must point
 * at the same File or the user will install a CA we later can't sign
 * leaves with.
 */
object ParvazDataDir {
    const val SUBDIR = "parvaz-data"

    fun forContext(context: Context): File =
        File(context.applicationContext.filesDir, SUBDIR).apply {
            if (!exists()) mkdirs()
        }
}
