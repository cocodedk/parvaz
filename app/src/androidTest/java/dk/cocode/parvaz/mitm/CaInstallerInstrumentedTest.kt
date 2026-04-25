package dk.cocode.parvaz.mitm

import android.content.Context
import android.security.KeyChain
import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith

/**
 * Validates the Android-side plumbing of CaInstaller on a real device
 * or emulator. The actual install flow needs user interaction so we
 * can't cover it end-to-end automatically — these tests pin down the
 * deterministic pieces: intent composition and fingerprint walking.
 */
@RunWith(AndroidJUnit4::class)
class CaInstallerInstrumentedTest {

    private lateinit var context: Context
    private lateinit var installer: CaInstaller

    @Before
    fun setUp() {
        context = ApplicationProvider.getApplicationContext()
        installer = CaInstaller(context)
    }

    @Test
    fun buildInstallIntent_hasCertificateAndName() {
        val der = byteArrayOf(1, 2, 3, 4, 5)
        val intent = installer.buildInstallIntent(der, name = "Parvaz Test CA")
        val extra = intent.getByteArrayExtra(KeyChain.EXTRA_CERTIFICATE)
        val name = intent.getStringExtra(KeyChain.EXTRA_NAME)
        assertNotNull(extra)
        assertEquals(5, extra!!.size)
        assertEquals(1.toByte(), extra[0])
        assertEquals("Parvaz Test CA", name)
    }

    @Test
    fun isInstalled_returnsFalseForUntrustedFingerprint() = runBlocking {
        // A random 256-byte DER blob whose SHA-256 will not match any
        // system or user CA currently trusted on the test device.
        val bogusDer = ByteArray(256) { (it * 17 + 5).toByte() }
        assertFalse(installer.isInstalled(bogusDer))
    }

    @Test
    fun isDeviceSecure_returnsBooleanWithoutCrashing() {
        // We don't assert true/false — the test device may or may not
        // have a screen lock. We assert only that the call terminates.
        installer.isDeviceSecure()
    }
}
