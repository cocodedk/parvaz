package dk.cocode.parvaz.ui.onboarding

import android.content.Context
import android.content.Intent
import android.net.Uri
import dk.cocode.parvaz.mitm.CaExporter
import dk.cocode.parvaz.mitm.CaFingerprint
import dk.cocode.parvaz.mitm.CaInstaller
import dk.cocode.parvaz.mitm.SettingsLauncher
import dk.cocode.parvaz.settings.ParvazDataDir
import dk.cocode.parvaz.vpn.CaGenerator
import java.io.File

/**
 * Orchestration for the CA install flow. Separated from the
 * composable so the moving parts (PEM generation, Downloads export,
 * Settings hand-off, fingerprint verification) stay testable and the
 * screen itself stays under the 200-line ceiling.
 *
 * No state of its own — the composable owns the state machine.
 * Controller methods are invoked from coroutines the composable owns.
 */
class CaInstallController(
    context: Context,
    private val generator: CaGenerator,
    private val installer: CaInstaller,
    private val exporter: CaExporter = CaExporter(context),
) {
    private val appContext = context.applicationContext

    suspend fun materialiseCA(): Result<ByteArray> {
        val dataDir = ParvazDataDir.forContext(appContext)
        return generator.generate(dataDir)
    }

    /**
     * Reload an existing on-disk CA. After process death while the user
     * was in system Settings installing the CA, the in-memory PEM is
     * gone but the file is still there — cheaper than re-running
     * parvazd -gen-ca.
     */
    fun loadPersistedCA(): ByteArray? {
        val f = File(ParvazDataDir.forContext(appContext), "ca/ca.crt")
        return f.takeIf { it.isFile }?.readBytes()
    }

    /**
     * Drop the PEM into a user-visible location so the system file
     * picker can browse to it. Returns the resulting content URI for
     * the secondary "Show file" CTA.
     */
    suspend fun export(caPem: ByteArray): Result<CaExporter.ExportedCa> =
        runCatching { exporter.export(caPem) }

    /** Closest-available Settings landing page; resolver-fallback chain. */
    fun buildSettingsIntent(): Intent =
        SettingsLauncher.buildSecurityIntent(appContext.packageManager)

    /** Direct ACTION_VIEW on the exported .crt's content URI. */
    fun buildShowFileIntent(uri: Uri): Intent =
        SettingsLauncher.buildViewCertFileIntent(uri)

    /**
     * After the system install flow returns, walk AndroidCAStore and
     * compare SHA-256 fingerprints. The activity-result code is
     * unreliable — this is the only trustworthy signal.
     */
    suspend fun isInstalled(caPem: ByteArray): Boolean =
        runCatching {
            val der = CaFingerprint.pemToDer(caPem)
            installer.isInstalled(der)
        }.getOrDefault(false)

    fun isDeviceSecure(): Boolean = installer.isDeviceSecure()
}
