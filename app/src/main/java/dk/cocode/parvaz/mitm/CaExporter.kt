package dk.cocode.parvaz.mitm

import android.content.ContentResolver
import android.content.ContentValues
import android.content.Context
import android.net.Uri
import android.os.Build
import android.os.Environment
import android.provider.MediaStore
import androidx.annotation.RequiresApi
import androidx.core.content.FileProvider
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.File
import java.io.IOException

/** MIME type for X.509 CA certificates — referenced by `SettingsLauncher` and tests. */
internal const val CA_MIME_TYPE = "application/x-x509-ca-cert"

/**
 * Suffix paired with `applicationId` to form the FileProvider authority
 * declared in `AndroidManifest.xml`. Keep the two in sync.
 */
internal const val FILE_PROVIDER_AUTHORITY_SUFFIX = ".fileprovider"

/**
 * Drops the MITM root CA on disk in a place the system file picker can
 * browse. `KeyChain.createInstallIntent()` no longer installs CA certs
 * on Android 11+ — apps must hand the cert to the user, who then picks
 * it from Settings → Install from device storage.
 *
 *  • API 29+: MediaStore.Downloads insert with IS_PENDING=1 during
 *    write, cleared after flush. Cert lands in the user's public
 *    Downloads folder.
 *  • API 24-28: app-scoped external Downloads + FileProvider URI for
 *    `ACTION_VIEW`. Avoids requesting WRITE_EXTERNAL_STORAGE — that
 *    runtime ask would alarm Iranian users.
 *
 * Pure I/O on Dispatchers.IO. No Compose deps. Idempotent — re-export
 * with the same displayName replaces the prior file.
 */
class CaExporter(
    context: Context,
    private val ioDispatcher: CoroutineDispatcher = Dispatchers.IO,
) {
    private val appContext = context.applicationContext

    data class ExportedCa(val displayPath: String, val contentUri: Uri)

    suspend fun export(
        pem: ByteArray,
        displayName: String = DEFAULT_FILENAME,
    ): ExportedCa = withContext(ioDispatcher) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            exportViaMediaStore(pem, displayName)
        } else {
            exportViaAppFiles(pem, displayName)
        }
    }

    @RequiresApi(Build.VERSION_CODES.Q)
    private fun exportViaMediaStore(pem: ByteArray, displayName: String): ExportedCa {
        val resolver = appContext.contentResolver
        deleteExisting(resolver, displayName)
        val values = ContentValues().apply {
            put(MediaStore.Downloads.DISPLAY_NAME, displayName)
            put(MediaStore.Downloads.MIME_TYPE, CA_MIME_TYPE)
            put(MediaStore.Downloads.RELATIVE_PATH, Environment.DIRECTORY_DOWNLOADS)
            put(MediaStore.Downloads.IS_PENDING, 1)
        }
        val uri = resolver.insert(MediaStore.Downloads.EXTERNAL_CONTENT_URI, values)
            ?: throw IOException("MediaStore.insert returned null for $displayName")
        try {
            resolver.openOutputStream(uri, "wt")?.use { out ->
                out.write(pem)
                out.flush()
            } ?: throw IOException("openOutputStream returned null")
            // Clear IS_PENDING so the file becomes visible to the system
            // picker. Without this Q+ devices hide it from "Install from
            // device storage".
            val clear = ContentValues().apply {
                put(MediaStore.Downloads.IS_PENDING, 0)
            }
            resolver.update(uri, clear, null, null)
        } catch (e: Exception) {
            // Best effort — try to clean up the half-written entry.
            runCatching { resolver.delete(uri, null, null) }
            throw e
        }
        return ExportedCa(
            displayPath = "Downloads/$displayName",
            contentUri = uri,
        )
    }

    private fun exportViaAppFiles(pem: ByteArray, displayName: String): ExportedCa {
        val downloadsDir = appContext.getExternalFilesDir(Environment.DIRECTORY_DOWNLOADS)
            ?: throw IOException("getExternalFilesDir(DIRECTORY_DOWNLOADS) returned null")
        downloadsDir.mkdirs()
        val out = File(downloadsDir, displayName)
        out.writeBytes(pem)
        val authority = "${appContext.packageName}$FILE_PROVIDER_AUTHORITY_SUFFIX"
        val uri = FileProvider.getUriForFile(appContext, authority, out)
        return ExportedCa(
            displayPath = "Android/data/${appContext.packageName}/files/Download/$displayName",
            contentUri = uri,
        )
    }

    /**
     * Delete any prior MediaStore entries under the same DISPLAY_NAME +
     * RELATIVE_PATH. Without this, repeated onboarding runs accumulate
     * `parvaz-ca (1).crt`, `parvaz-ca (2).crt`, etc. — Android disambiguates
     * by suffix on duplicate names instead of overwriting.
     */
    @RequiresApi(Build.VERSION_CODES.Q)
    private fun deleteExisting(resolver: ContentResolver, displayName: String) {
        val selection = "${MediaStore.Downloads.DISPLAY_NAME} = ? AND " +
            "${MediaStore.Downloads.RELATIVE_PATH} LIKE ?"
        val args = arrayOf(displayName, "${Environment.DIRECTORY_DOWNLOADS}%")
        runCatching {
            resolver.delete(
                MediaStore.Downloads.EXTERNAL_CONTENT_URI,
                selection,
                args,
            )
        }
    }

    companion object {
        const val DEFAULT_FILENAME = "parvaz-ca.crt"
    }
}
