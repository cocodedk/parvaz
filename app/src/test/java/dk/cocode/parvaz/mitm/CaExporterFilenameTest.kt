package dk.cocode.parvaz.mitm

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class CaExporterFilenameTest {
    @Test
    fun defaultFilenameIncludesCertificateFingerprintPrefix() {
        val pem = javaClass.classLoader!!
            .getResourceAsStream("mitm/test_ca.pem")!!
            .use { it.readBytes() }

        val der = CaFingerprint.pemToDer(pem)
        val expectedPrefix = CaFingerprint.sha256(der)
            .joinToString(separator = "") { "%02x".format(it.toInt() and 0xff) }
            .take(12)

        val filename = CaExporter.defaultFilename(pem)

        assertEquals("parvaz-ca-$expectedPrefix.crt", filename)
        assertTrue(filename != "parvaz-ca.crt")
    }
}
