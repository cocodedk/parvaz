package dk.cocode.parvaz.update

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class GitHubReleasesClientTest {

    private fun fixture(name: String): String =
        javaClass.classLoader!!.getResourceAsStream(name)!!
            .bufferedReader().use { it.readText() }

    @Test fun parsesLatestPayload_extractsTagAndBody() {
        val release = GitHubReleasesClient.parseRelease(fixture("release_v0.1.4.json"))
        assertNotNull(release)
        assertEquals("v0.1.4", release!!.tagName)
        assertEquals(Version(0, 1, 4), release.version)
        assertTrue("body should contain release notes", release.body.contains("perf:"))
    }

    @Test fun extractsApkAssetUrl() {
        val release = GitHubReleasesClient.parseRelease(fixture("release_v0.1.4.json"))!!
        assertEquals(
            "https://github.com/cocodedk/parvaz/releases/download/v0.1.4/Parvaz.apk",
            release.apkUrl,
        )
        assertEquals(13_632_771L, release.apkSizeBytes)
    }

    @Test fun extractsSha256AssetUrl() {
        val release = GitHubReleasesClient.parseRelease(fixture("release_v0.1.4.json"))!!
        assertEquals(
            "https://github.com/cocodedk/parvaz/releases/download/v0.1.4/Parvaz.apk.sha256",
            release.sha256Url,
        )
    }

    @Test fun parseReleaseReturnsNullOnGarbage() {
        assertNull(GitHubReleasesClient.parseRelease("not json"))
        assertNull(GitHubReleasesClient.parseRelease("{}"))
    }

    @Test fun parseReleaseReturnsNullWhenApkAssetMissing() {
        // Same payload but with the APK asset stripped out.
        val raw = fixture("release_v0.1.4.json")
            .replace("\"name\":\"Parvaz.apk\"", "\"name\":\"NotApk.apk\"")
        assertNull(GitHubReleasesClient.parseRelease(raw))
    }

    @Test fun parseReleaseResult_DistinguishesNoAssetFromMalformed() {
        val malformed = GitHubReleasesClient.parseReleaseResult("not json")
        assertEquals(FetchResult.Malformed, malformed)

        val noAsset = GitHubReleasesClient.parseReleaseResult(
            fixture("release_v0.1.4.json")
                .replace("\"name\":\"Parvaz.apk\"", "\"name\":\"NotApk.apk\""),
        )
        assertEquals(FetchResult.NoAsset, noAsset)
    }

    @Test fun parseReleaseResult_TreatsZeroSizedApkAsNoAsset() {
        // A release JSON where the Parvaz.apk asset advertises size=0 must
        // surface as NoAsset rather than Success — otherwise downstream
        // progress UI and pre-flight free-space checks degrade silently.
        val raw = fixture("release_v0.1.4.json")
            .replace("\"size\":13632771", "\"size\":0")
        assertEquals(FetchResult.NoAsset, GitHubReleasesClient.parseReleaseResult(raw))
    }

    @Test fun parseReleaseResult_SuccessOnRealPayload() {
        val r = GitHubReleasesClient.parseReleaseResult(fixture("release_v0.1.4.json"))
        check(r is FetchResult.Success) { "expected Success, got $r" }
        assertEquals("v0.1.4", r.release.tagName)
    }
}
