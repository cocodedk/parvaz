package dk.cocode.parvaz.update

import java.io.File
import java.io.IOException
import java.io.InputStream
import java.net.HttpURLConnection
import java.net.URL
import java.security.MessageDigest

sealed interface ApkDownloadOutcome {
    data class Success(val file: File, val sha256: String) : ApkDownloadOutcome
    data object ShaMismatch : ApkDownloadOutcome
    data class NetworkError(val cause: Throwable) : ApkDownloadOutcome
}

/**
 * Streams the APK from GitHub Releases to disk and verifies its
 * SHA-256 against the companion `.sha256` asset before reporting success.
 *
 * The file is **deleted on any failure** (mismatch, IO, cancellation) —
 * we never leave a partial APK around because the system PackageInstaller
 * will refuse to verify it and the user is left wondering what happened.
 *
 * Network access is parameterized via [opener] so unit tests can feed
 * canned bytes without sockets. In production [openConnection] composes
 * an `HttpURLConnection.getInputStream`. Connection pooling is whatever
 * `HttpURLConnection` does by default — we don't tune it because each
 * download is one shot.
 */
class ApkDownloader(
    private val opener: (String) -> InputStream = { url ->
        val conn = URL(url).openConnection() as HttpURLConnection
        conn.connectTimeout = 15_000
        conn.readTimeout = 30_000
        conn.setRequestProperty("User-Agent", "parvaz-app")
        if (conn.responseCode !in 200..299) {
            throw IOException("HTTP ${conn.responseCode} for $url")
        }
        conn.inputStream
    },
) {

    /**
     * Downloads to [destination], verifies SHA-256 from [sha256Url], and
     * returns one of [ApkDownloadOutcome]. [onProgress] fires every
     * [PROGRESS_TICK_BYTES] and once at completion. [totalBytes], when
     * known, lets the UI render a percentage; otherwise progress reports
     * 0 as total.
     */
    fun download(
        apkUrl: String,
        sha256Url: String,
        destination: File,
        totalBytes: Long = 0L,
        onProgress: (downloaded: Long, total: Long) -> Unit,
    ): ApkDownloadOutcome {
        return try {
            val expected = readExpectedSha(sha256Url)
            val actual = streamToFile(apkUrl, destination, totalBytes, onProgress)
            if (!actual.equals(expected, ignoreCase = true)) {
                destination.delete()
                ApkDownloadOutcome.ShaMismatch
            } else {
                ApkDownloadOutcome.Success(destination, actual)
            }
        } catch (e: Exception) {
            destination.delete()
            ApkDownloadOutcome.NetworkError(e)
        }
    }

    private fun readExpectedSha(sha256Url: String): String {
        val raw = opener(sha256Url).bufferedReader().use { it.readText() }
        // Format is either bare hex or `<hex>  filename` (GitHub style).
        val token = raw.trim().substringBefore(' ').substringBefore('\t')
        require(token.length == 64) { "expected 64-char hex sha256, got ${token.length}" }
        return token.lowercase()
    }

    private fun streamToFile(
        apkUrl: String,
        destination: File,
        totalBytes: Long,
        onProgress: (Long, Long) -> Unit,
    ): String {
        val digest = MessageDigest.getInstance("SHA-256")
        var written = 0L
        var sinceLastTick = 0L
        opener(apkUrl).use { input ->
            destination.outputStream().use { out ->
                val buf = ByteArray(BUFFER_BYTES)
                while (true) {
                    val n = input.read(buf)
                    if (n < 0) break
                    out.write(buf, 0, n)
                    digest.update(buf, 0, n)
                    written += n
                    sinceLastTick += n
                    if (sinceLastTick >= PROGRESS_TICK_BYTES) {
                        onProgress(written, totalBytes)
                        sinceLastTick = 0L
                    }
                }
            }
        }
        onProgress(written, if (totalBytes > 0) totalBytes else written)
        return digest.digest().joinToString("") { "%02x".format(it) }
    }

    companion object {
        private const val BUFFER_BYTES = 8 * 1024
        private const val PROGRESS_TICK_BYTES = 64 * 1024
    }
}
