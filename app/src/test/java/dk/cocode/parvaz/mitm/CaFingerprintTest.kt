package dk.cocode.parvaz.mitm

import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertThrows
import org.junit.Test

/**
 * Unit tests for the pure PEM parsing + fingerprint helpers. Uses a
 * throwaway ECDSA P-256 self-signed cert stored as a test resource —
 * same shape as what the Go side's `parvazd -gen-ca` emits. Fingerprint
 * behaviour is algorithm-agnostic so a fixture is enough.
 */
class CaFingerprintTest {

    private val samplePem: ByteArray by lazy {
        javaClass.getResourceAsStream("/mitm/test_ca.pem")
            ?.use { it.readBytes() }
            ?: error("test_ca.pem fixture missing")
    }

    @Test
    fun pemToDer_roundTripStable() {
        val a = CaFingerprint.pemToDer(samplePem)
        val b = CaFingerprint.pemToDer(samplePem)
        assertArrayEquals("same PEM always decodes to the same DER", a, b)
        assert(a.size in 200..1000) { "DER size unexpected: ${a.size}" }
    }

    @Test
    fun sha256_is32BytesAndDeterministic() {
        val a = CaFingerprint.fingerprint(samplePem)
        val b = CaFingerprint.fingerprint(samplePem)
        assertEquals(32, a.size)
        assertArrayEquals(a, b)
    }

    @Test
    fun sha256_changesWhenInputChanges() {
        val a = CaFingerprint.sha256(byteArrayOf(1, 2, 3))
        val b = CaFingerprint.sha256(byteArrayOf(1, 2, 4))
        assertNotEquals(a.toList(), b.toList())
    }

    @Test
    fun pemToDer_rejectsNonCertificateInput() {
        assertThrows(Exception::class.java) {
            CaFingerprint.pemToDer("not a certificate".toByteArray())
        }
    }
}
