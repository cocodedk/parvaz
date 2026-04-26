package dk.cocode.parvaz.mitm

import android.app.KeyguardManager
import android.content.Context
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.security.KeyStore

/**
 * Android-side helpers around the user CA store.
 *
 *  • [isDeviceSecure] — Android refuses CA install when no screen lock
 *    is set. Pre-check before opening Settings so the user hits a
 *    Farsi error instead of a confused settings screen.
 *  • [isInstalled] — walks `AndroidCAStore`, comparing SHA-256 of each
 *    alias's encoded certificate. The system Settings install flow's
 *    activity-result code is unreliable across OEMs, so this
 *    fingerprint check is the only trustworthy confirmation that the
 *    user actually completed the manual install.
 *
 * Cert export and Settings hand-off live in [CaExporter] and
 * [SettingsLauncher] respectively. `KeyChain.createInstallIntent`
 * is no longer used — it stopped installing CA certificates on
 * Android 11+ and silently dropped the cert extras.
 *
 * Instrumentation-only — KeyguardManager + AndroidCAStore require a
 * real Android runtime. See `CaInstallerInstrumentedTest`.
 */
class CaInstaller(
    context: Context,
    private val ioDispatcher: CoroutineDispatcher = Dispatchers.IO,
) {
    private val appContext = context.applicationContext

    fun isDeviceSecure(): Boolean {
        val km = appContext.getSystemService(Context.KEYGUARD_SERVICE) as? KeyguardManager
            ?: return false
        return km.isDeviceSecure
    }

    /**
     * True if a certificate matching [expectedDer]'s SHA-256 fingerprint
     * is currently trusted under `AndroidCAStore` (unified view of system
     * + user CAs). Walks up to ~150 aliases — call off the main thread;
     * the default dispatcher is `Dispatchers.IO`.
     */
    suspend fun isInstalled(expectedDer: ByteArray): Boolean = withContext(ioDispatcher) {
        val expectedSha = CaFingerprint.sha256(expectedDer)
        val ks = KeyStore.getInstance(ANDROID_CA_STORE).apply { load(null) }
        val aliases = ks.aliases()
        while (aliases.hasMoreElements()) {
            val alias = aliases.nextElement()
            val cert = ks.getCertificate(alias) ?: continue
            if (CaFingerprint.sha256(cert.encoded).contentEquals(expectedSha)) {
                return@withContext true
            }
        }
        false
    }

    companion object {
        private const val ANDROID_CA_STORE = "AndroidCAStore"
    }
}
