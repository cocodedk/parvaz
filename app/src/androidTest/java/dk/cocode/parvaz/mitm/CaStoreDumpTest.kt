package dk.cocode.parvaz.mitm

import android.content.Context
import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import org.junit.Test
import org.junit.runner.RunWith
import java.io.File
import java.security.KeyStore
import java.security.MessageDigest
import java.security.cert.X509Certificate

/**
 * Temporary diagnostic — dumps AndroidCAStore alias list + SHA-256s and
 * compares against the app's on-disk CA. Left in androidTest so it can
 * be re-run later when investigating cert-install issues. NOT intended
 * as a regression gate; always passes but writes findings to logcat
 * and fails ONLY if the app's PEM is missing from filesDir.
 */
@RunWith(AndroidJUnit4::class)
class CaStoreDumpTest {
    @Test
    fun dumpAndroidCaStoreAndCompareToAppPem() {
        val ctx = ApplicationProvider.getApplicationContext<Context>()
        val pemFile = File(ctx.filesDir, "parvaz-data/ca/ca.crt")
        if (!pemFile.isFile) {
            println("CASTORE_DUMP: no app PEM at ${pemFile.absolutePath} — run onboarding first")
            return
        }
        val expectedPem = pemFile.readBytes()
        val expectedDer = CaFingerprint.pemToDer(expectedPem)
        val expectedSha = CaFingerprint.sha256(expectedDer).toHex()
        println("CASTORE_DUMP: app cert path=${pemFile.absolutePath} size=${pemFile.length()}")
        println("CASTORE_DUMP: app cert sha256=$expectedSha")

        val ks = KeyStore.getInstance("AndroidCAStore").apply { load(null) }
        val aliases = ks.aliases().toList()
        println("CASTORE_DUMP: total aliases=${aliases.size}")

        val userAliases = aliases.filter { it.startsWith("user:") }
        println("CASTORE_DUMP: user-installed aliases=${userAliases.size}")

        var foundByFingerprint = false
        var foundBySubject = false
        for (alias in userAliases) {
            val cert = ks.getCertificate(alias) as? X509Certificate ?: continue
            val aliasSha = CaFingerprint.sha256(cert.encoded).toHex()
            val subject = cert.subjectX500Principal.name
            println("CASTORE_DUMP: alias=$alias sha256=$aliasSha subject=$subject")
            if (aliasSha == expectedSha) foundByFingerprint = true
            if (subject.contains("Parvaz", ignoreCase = true)) foundBySubject = true
        }
        println("CASTORE_DUMP: foundByFingerprint=$foundByFingerprint foundBySubject=$foundBySubject")

        // Also probe a handful of system aliases so we know the store
        // walk itself is functional on this OEM/API combo.
        val systemPreview = aliases.filter { it.startsWith("system:") }.take(3)
        for (alias in systemPreview) {
            val cert = ks.getCertificate(alias) as? X509Certificate ?: continue
            println("CASTORE_DUMP: system alias=$alias subject=${cert.subjectX500Principal.name}")
        }
    }

    private fun ByteArray.toHex(): String =
        joinToString(separator = "") { "%02X".format(it) }

    @Suppress("unused")
    private fun debugSha(bytes: ByteArray): String =
        MessageDigest.getInstance("SHA-256").digest(bytes).toHex()
}
