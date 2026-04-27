package dk.cocode.parvaz.update

import org.junit.After
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.ByteArrayInputStream
import java.io.File
import java.io.IOException
import java.io.InputStream
import java.security.MessageDigest

class ApkDownloaderTest {

    @get:Rule val tmp = TemporaryFolder()

    private lateinit var dest: File

    @Before fun setup() {
        dest = File(tmp.root, "Parvaz.apk")
    }

    @After fun cleanup() {
        if (dest.exists()) dest.delete()
    }

    private fun sha256(bytes: ByteArray): String =
        MessageDigest.getInstance("SHA-256").digest(bytes)
            .joinToString("") { "%02x".format(it) }

    private fun openerForBody(apk: ByteArray, sha256Body: String): (String) -> InputStream =
        fun(url: String): InputStream {
            return when {
                url.endsWith(".apk") -> ByteArrayInputStream(apk)
                url.endsWith(".sha256") -> ByteArrayInputStream(sha256Body.toByteArray())
                else -> throw IllegalArgumentException("unexpected url: $url")
            }
        }

    @Test fun streamsApkToFileWithMatchingSha() {
        val apk = ByteArray(1024) { (it and 0xff).toByte() }
        val expected = sha256(apk)
        val downloader = ApkDownloader(opener = openerForBody(apk, "$expected  Parvaz.apk\n"))

        val outcome = downloader.download(
            apkUrl = "https://example.test/Parvaz.apk",
            sha256Url = "https://example.test/Parvaz.apk.sha256",
            destination = dest,
            onProgress = { _, _ -> },
        )

        assertTrue(outcome is ApkDownloadOutcome.Success)
        assertTrue(dest.exists())
        assertArrayEquals(apk, dest.readBytes())
    }

    @Test fun rejectsMismatchedSha_andDeletesPartial() {
        val apk = ByteArray(512) { 0x42 }
        val downloader = ApkDownloader(opener = openerForBody(apk, "0".repeat(64) + "  Parvaz.apk\n"))

        val outcome = downloader.download(
            apkUrl = "https://example.test/Parvaz.apk",
            sha256Url = "https://example.test/Parvaz.apk.sha256",
            destination = dest,
            onProgress = { _, _ -> },
        )

        assertTrue("expected ShaMismatch, got $outcome", outcome is ApkDownloadOutcome.ShaMismatch)
        assertFalse("partial file must be deleted", dest.exists())
    }

    @Test fun emitsProgressUpToTotal() {
        val apk = ByteArray(64 * 1024) { 0x21 }
        val expected = sha256(apk)
        val downloader = ApkDownloader(opener = openerForBody(apk, "$expected  Parvaz.apk\n"))
        val progressTicks = mutableListOf<Pair<Long, Long>>()

        downloader.download(
            apkUrl = "https://example.test/Parvaz.apk",
            sha256Url = "https://example.test/Parvaz.apk.sha256",
            destination = dest,
            totalBytes = apk.size.toLong(),
            onProgress = { downloaded, total -> progressTicks += downloaded to total },
        )

        assertTrue("progress should fire at least once", progressTicks.isNotEmpty())
        val (lastDone, lastTotal) = progressTicks.last()
        assertEquals(apk.size.toLong(), lastDone)
        assertEquals(apk.size.toLong(), lastTotal)
    }

    @Test fun networkErrorReturnsErrorAndCleansPartial() {
        val downloader = ApkDownloader(opener = { throw IOException("boom") })

        val outcome = downloader.download(
            apkUrl = "https://example.test/Parvaz.apk",
            sha256Url = "https://example.test/Parvaz.apk.sha256",
            destination = dest,
            onProgress = { _, _ -> },
        )

        assertTrue("expected NetworkError, got $outcome", outcome is ApkDownloadOutcome.NetworkError)
        assertFalse(dest.exists())
    }

    @Test fun parsesShaFileWithFilenameSuffix() {
        // GitHub-style sha256 file: "<hex>  Parvaz.apk\n"
        val apk = ByteArray(32) { 0x7f }
        val expected = sha256(apk)
        val downloader = ApkDownloader(opener = openerForBody(apk, "$expected  Parvaz.apk\n"))

        val outcome = downloader.download(
            apkUrl = "https://example.test/Parvaz.apk",
            sha256Url = "https://example.test/Parvaz.apk.sha256",
            destination = dest,
            onProgress = { _, _ -> },
        )
        assertTrue(outcome is ApkDownloadOutcome.Success)
    }

    @Test fun parsesBareHexShaFile() {
        val apk = ByteArray(8) { 0x09 }
        val expected = sha256(apk)
        val downloader = ApkDownloader(opener = openerForBody(apk, expected))

        val outcome = downloader.download(
            apkUrl = "https://example.test/Parvaz.apk",
            sha256Url = "https://example.test/Parvaz.apk.sha256",
            destination = dest,
            onProgress = { _, _ -> },
        )
        assertNotNull(outcome)
        assertTrue(outcome is ApkDownloadOutcome.Success)
    }
}
