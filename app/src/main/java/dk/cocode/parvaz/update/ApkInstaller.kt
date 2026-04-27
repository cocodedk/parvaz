package dk.cocode.parvaz.update

import android.content.Context
import android.content.Intent
import android.net.Uri
import android.os.Build
import android.provider.Settings
import androidx.core.content.FileProvider
import java.io.File

sealed interface InstallOutcome {
    /** System installer was launched. The user takes over from here. */
    data object Launched : InstallOutcome

    /** App lacks REQUEST_INSTALL_PACKAGES authorization at runtime.
     *  We launched ACTION_MANAGE_UNKNOWN_APP_SOURCES; the user must
     *  toggle it on and tap Install again. */
    data object NeedsUnknownSourcesPermission : InstallOutcome
}

/**
 * Hands the downloaded APK to Android's system PackageInstaller via
 * `ACTION_VIEW` + `application/vnd.android.package-archive`. The
 * underlying file URI is exposed through the project's FileProvider
 * (`${applicationId}.fileprovider`), entry `apk_update`, mapped to
 * `cacheDir/parvaz-update/`.
 *
 * On API 26+ the install will silently no-op until the user grants
 * `REQUEST_INSTALL_PACKAGES` for our package — we deep-link to the
 * system "install unknown apps" settings page in that case and let
 * the user retry.
 *
 * Does not handle the install result. The system installer takes over
 * the screen; once the new APK is committed, our process is replaced
 * by the new package and the in-flight "DOWNLOADING" UI is gone with
 * it. If the user cancels the system dialog, they're returned to our
 * (stale) settings sheet, which is fine — they can re-tap Install.
 */
class ApkInstaller(private val context: Context) {

    fun install(apk: File): InstallOutcome {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O &&
            !context.packageManager.canRequestPackageInstalls()) {
            launchUnknownSourcesSettings()
            return InstallOutcome.NeedsUnknownSourcesPermission
        }

        val authority = "${context.packageName}.fileprovider"
        val uri: Uri = FileProvider.getUriForFile(context, authority, apk)
        val intent = Intent(Intent.ACTION_VIEW).apply {
            setDataAndType(uri, "application/vnd.android.package-archive")
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
        }
        context.startActivity(intent)
        return InstallOutcome.Launched
    }

    private fun launchUnknownSourcesSettings() {
        val intent = Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES).apply {
            data = Uri.parse("package:${context.packageName}")
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        }
        context.startActivity(intent)
    }

    /**
     * Stable destination for downloaded APKs. Caller should mkdir before
     * writing. Cleared by the OS as part of cache eviction.
     */
    fun destinationFile(): File {
        val dir = File(context.cacheDir, "parvaz-update")
        if (!dir.exists()) dir.mkdirs()
        return File(dir, "Parvaz.apk")
    }
}
