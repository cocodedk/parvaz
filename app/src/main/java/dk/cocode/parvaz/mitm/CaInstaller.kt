package dk.cocode.parvaz.mitm

import android.app.KeyguardManager
import android.content.Context
import android.content.Intent
import android.security.KeyChain
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.security.KeyStore

/**
 * CaInstaller bridges the MITM root CA into Android's user-CA store.
 *  1. [isDeviceSecure] — Android refuses CA install when no screen lock
 *     is set. Check before opening the intent so the user hits a Farsi
 *     error instead of a confused settings screen.
 *  2. [buildInstallIntent] — DER bytes + friendly name, handed to
 *     `KeyChain.createInstallIntent()`.
 *  3. [isInstalled] — walks `AndroidCAStore`, comparing SHA-256 of each
 *     alias's encoded certificate. The activity-result code is
 *     unreliable across OEMs, so this fingerprint check is the only
 *     trustworthy confirmation the user actually tapped *install*.
 *
 * Instrumentation-only — KeyChain + AndroidCAStore require a real
 * Android runtime. See `CaInstallerInstrumentedTest`.
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
     * Build the system intent that pops Settings' "Install certificate"
     * flow. [caDer] must be the DER bytes of an X.509 CA certificate —
     * convert from PEM via [CaFingerprint.pemToDer] first.
     */
    fun buildInstallIntent(caDer: ByteArray, name: String = DEFAULT_NAME): Intent =
        KeyChain.createInstallIntent().apply {
            putExtra(KeyChain.EXTRA_CERTIFICATE, caDer)
            putExtra(KeyChain.EXTRA_NAME, name)
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
        /** Shown to the user by Android's install dialog. Matches the
         *  Go-side CommonName at core/mitm/ca.go. */
        const val DEFAULT_NAME = "Parvaz Root CA"
        private const val ANDROID_CA_STORE = "AndroidCAStore"
    }
}
