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
 * Outcome of [GitHubReleasesClient.fetchLatest] / [GitHubReleasesClient.parseRelease].
 * Distinguishes "no APK asset on the latest release" from "couldn't reach
 * GitHub" so the UI can surface the right error string to the user.
 */
sealed interface FetchResult {
    data class Success(val release: ReleaseInfo) : FetchResult
    /** Release JSON parsed but Parvaz.apk / Parvaz.apk.sha256 missing or zero-sized. */
    data object NoAsset : FetchResult
    /** Network I/O / non-2xx / DNS / cert failure. */
    data object NetworkError : FetchResult
    /** Body wasn't valid release JSON. Surfaced as NetworkError to the UI today. */
    data object Malformed : FetchResult
}

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
     * Fetches the latest release. Returns a [FetchResult] — never throws
     * for ordinary network/URL/parse errors, the UI layer maps the
     * error variants onto Farsi strings.
     */
    fun fetchLatest(): FetchResult {
        var conn: HttpURLConnection? = null
        return try {
            val url = URL("https://api.github.com/repos/$owner/$repo/releases/latest")
            conn = openConnection(url)
            conn.requestMethod = "GET"
            conn.connectTimeout = TIMEOUT_MS
            conn.readTimeout = TIMEOUT_MS
            conn.setRequestProperty("Accept", "application/vnd.github+json")
            conn.setRequestProperty("User-Agent", USER_AGENT)
            if (conn.responseCode !in 200..299) FetchResult.NetworkError
            else parseReleaseResult(conn.inputStream.bufferedReader().use { it.readText() })
        } catch (_: Exception) {
            FetchResult.NetworkError
        } finally {
            conn?.disconnect()
        }
    }

    companion object {
        private const val USER_AGENT = "parvaz-app"
        private const val TIMEOUT_MS = 10_000

        /**
         * Backwards-compatible nullable parse helper. Returns the
         * release on success and null on any failure (malformed,
         * missing asset, zero-sized asset). Tests use this; production
         * code should prefer [parseReleaseResult] for the granular
         * outcome.
         */
        fun parseRelease(raw: String): ReleaseInfo? =
            (parseReleaseResult(raw) as? FetchResult.Success)?.release

        /**
         * Granular variant. Treats a non-positive `size` field as a
         * parse failure (NoAsset) so the caller's pre-flight checks
         * don't see a "0-byte APK" and break.
         */
        fun parseReleaseResult(raw: String): FetchResult = try {
            val obj = JSONObject(raw)
            val tag = obj.optString("tag_name").takeIf { it.isNotEmpty() }
                ?: return FetchResult.Malformed
            val version = Version.parse(tag) ?: return FetchResult.Malformed
            val body = obj.optString("body", "")
            val assets = obj.optJSONArray("assets") ?: return FetchResult.Malformed

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
            if (apkUrl == null || shaUrl == null || apkSize <= 0L) return FetchResult.NoAsset

            FetchResult.Success(
                ReleaseInfo(
                    tagName = tag,
                    version = version,
                    body = body,
                    apkUrl = apkUrl,
                    apkSizeBytes = apkSize,
                    sha256Url = shaUrl,
                ),
            )
        } catch (_: Exception) {
            FetchResult.Malformed
        }
    }
}
