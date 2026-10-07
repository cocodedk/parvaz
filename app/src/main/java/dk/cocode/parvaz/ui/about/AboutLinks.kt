package dk.cocode.parvaz.ui.about

/** The buttons on the About page that open a web page. */
enum class AboutLink { Updates, Privacy, Website, Source, Issues }

private const val APPLICATION_ID = "dk.cocode.parvaz"
private const val REPO = "https://github.com/cocodedk/parvaz"
private const val SITE = "https://parvaz.cocode.dk/"

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
 * and only when [privacyUrl] is null). The Persian site lives under /fa/; every other language
 * opens the English site. The app itself never checks for updates over the network: the update
 * button only opens a page in the browser.
 */
fun aboutUrl(
    link: AboutLink,
    language: String,
    fdroidLive: Boolean = FDROID_LIVE,
    privacyUrl: String? = PRIVACY_URL,
): String? = when (link) {
    AboutLink.Updates ->
        if (fdroidLive) "https://f-droid.org/packages/$APPLICATION_ID/" else "$REPO/releases/latest"
    AboutLink.Privacy -> privacyUrl
    AboutLink.Website -> if (language == "fa") SITE + "fa/" else SITE
    AboutLink.Source -> REPO
    AboutLink.Issues -> "$REPO/issues"
}
