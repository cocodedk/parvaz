package dk.cocode.parvaz.ui.about

/** The buttons on the About page that open a web page. */
enum class AboutLink { Updates, Privacy, Website, Source, Issues }

private const val APPLICATION_ID = "dk.cocode.parvaz"
private const val REPO = "https://github.com/cocodedk/parvaz"
private const val SITE = "https://parvaz.cocode.dk/"

/**
 * Languages the site has both a home page and a privacy page for, at `<site>/<code>/` and
 * `<site>/<code>/privacy/`. Persian has a home page (`/fa/`) but no privacy page of its own, so
 * Persian stays on the English pages; add "fa" here once `/fa/privacy/` exists.
 */
private val SITE_LANGUAGES = setOf("da")

/** [url] on the site in [language], or [url] itself when the site has no pages in that language. */
private fun inLanguage(url: String, language: String): String =
    if (language in SITE_LANGUAGES && url.startsWith(SITE)) "$SITE$language/${url.removePrefix(SITE)}" else url

/**
 * True once Parvaz is on F-Droid. apps.yml says `fdroid: mr:49559`, a merge request that is still
 * open, so for now the update button opens the latest GitHub release. Set this to true when the
 * app is live on F-Droid.
 */
const val FDROID_LIVE = false

/**
 * The privacy policy's address (apps.yml `privacy`), published by the privacy-policy PR (#110).
 * Merge that PR before this one. With null here the "Read the privacy policy" button is left out.
 */
val PRIVACY_URL: String? = "https://parvaz.cocode.dk/privacy/"

/**
 * Where an About-page button leads, or null when there is nothing to open (only the privacy link,
 * and only when [privacyUrl] is null). The website and privacy links follow [language] (the app's
 * own language setting, a code such as "da") and open the English pages when the site has none in
 * that language. The app itself never checks for updates over the network: the update button only
 * opens a page in the browser.
 */
fun aboutUrl(
    link: AboutLink,
    language: String,
    fdroidLive: Boolean = FDROID_LIVE,
    privacyUrl: String? = PRIVACY_URL,
): String? = when (link) {
    AboutLink.Updates ->
        if (fdroidLive) "https://f-droid.org/packages/$APPLICATION_ID/" else "$REPO/releases/latest"
    AboutLink.Privacy -> privacyUrl?.let { inLanguage(it, language) }
    AboutLink.Website -> inLanguage(SITE, language)
    AboutLink.Source -> REPO
    AboutLink.Issues -> "$REPO/issues"
}
