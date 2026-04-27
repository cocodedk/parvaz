package dk.cocode.parvaz.update

import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.URL

/**
 * Information about a GitHub release as parsed from the v3 REST API.
 * Only the four fields the updater actually consumes — we deliberately
 * don't expose the full asset list.
 */
data class ReleaseInfo(
    val tagName: String,
    val version: Version,
    val body: String,
    val apkUrl: String,
    val apkSizeBytes: Long,
    val sha256Url: String,
)

/**
 * Thin wrapper around `https://api.github.com/repos/<owner>/<repo>/releases/latest`.
 *
 * Anonymous (no auth header) — that's 60 requests per IP per hour, fine
 * for manual update checks. We deliberately do NOT cache, send User-Agent
 * `parvaz-app`, and time out aggressively so a slow GitHub edge doesn't
 * freeze the settings sheet.
 *
 * The JSON parsing entry point [parseRelease] is exposed as a static
 * function so unit tests can feed canned payloads without a network.
 */
class GitHubReleasesClient(
    private val owner: String = "cocodedk",
    private val repo: String = "parvaz",
    private val openConnection: (URL) -> HttpURLConnection = { it.openConnection() as HttpURLConnection },
) {

    /**
     * Fetches the latest release. Returns null on any network or parse
     * failure — the UI layer translates null into a Farsi error string.
     */
    fun fetchLatest(): ReleaseInfo? {
        val url = URL("https://api.github.com/repos/$owner/$repo/releases/latest")
        val conn = openConnection(url)
        return try {
            conn.requestMethod = "GET"
            conn.connectTimeout = TIMEOUT_MS
            conn.readTimeout = TIMEOUT_MS
            conn.setRequestProperty("Accept", "application/vnd.github+json")
            conn.setRequestProperty("User-Agent", USER_AGENT)
            if (conn.responseCode !in 200..299) return null
            val raw = conn.inputStream.bufferedReader().use { it.readText() }
            parseRelease(raw)
        } catch (_: Exception) {
            null
        } finally {
            conn.disconnect()
        }
    }

    companion object {
        private const val USER_AGENT = "parvaz-app"
        private const val TIMEOUT_MS = 10_000

        fun parseRelease(raw: String): ReleaseInfo? = try {
            val obj = JSONObject(raw)
            val tag = obj.optString("tag_name").takeIf { it.isNotEmpty() } ?: return null
            val version = Version.parse(tag) ?: return null
            val body = obj.optString("body", "")
            val assets = obj.optJSONArray("assets") ?: return null

            var apkUrl: String? = null
            var apkSize: Long = 0L
            var shaUrl: String? = null
            for (i in 0 until assets.length()) {
                val asset = assets.getJSONObject(i)
                val name = asset.optString("name")
                val download = asset.optString("browser_download_url")
                when {
                    name == "Parvaz.apk" -> {
                        apkUrl = download
                        apkSize = asset.optLong("size", 0L)
                    }
                    name == "Parvaz.apk.sha256" -> shaUrl = download
                }
            }
            if (apkUrl == null || shaUrl == null) return null

            ReleaseInfo(
                tagName = tag,
                version = version,
                body = body,
                apkUrl = apkUrl,
                apkSizeBytes = apkSize,
                sha256Url = shaUrl,
            )
        } catch (_: Exception) {
            null
        }
    }
}
