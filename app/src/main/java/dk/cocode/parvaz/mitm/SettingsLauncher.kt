package dk.cocode.parvaz.mitm

import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.provider.Settings

/**
 * Builds the Intents we hand off to the system from the CA install
 * screen. Two distinct hand-offs:
 *
 *  • [buildSecurityIntent] — opens Android Settings as close as we can
 *    get to "Install certificate from file". There is no public action
 *    that deep-links directly to the install dialog (the CertInstaller
 *    activity isn't exported), so we pick the closest landing page and
 *    fall back to top-level Settings if no resolver answers.
 *  • [buildViewCertFileIntent] — direct ACTION_VIEW on the content URI
 *    of the exported `.crt`. Some launchers will offer "Open with"
 *    Files / Documents apps, helping the user confirm the file landed
 *    where we said it did.
 *
 * Pure functions — no Context, easily fakeable in unit tests by
 * swapping the [PackageManager].
 */
object SettingsLauncher {

    /**
     * Resolver chain for the "open Settings" CTA, in priority order.
     * AOSP exposes no public action for the CertInstaller activity, so
     * the best we can do is land the user on a security-adjacent page
     * and let our on-screen guidance cover the rest of the navigation.
     */
    private val SECURITY_ACTIONS = listOf(
        Settings.ACTION_SECURITY_SETTINGS,
        // Some OEMs route credential install under Privacy.
        "android.settings.PRIVACY_SETTINGS",
        Settings.ACTION_SETTINGS,
    )

    /**
     * First action in [SECURITY_ACTIONS] that a real resolver answers
     * for. Wrapped in `Intent.FLAG_ACTIVITY_NEW_TASK` because the
     * caller may launch from a non-activity context in some compose
     * lifecycles.
     */
    fun buildSecurityIntent(packageManager: PackageManager): Intent {
        val action = pickSecurityAction { candidate ->
            packageManager.resolveActivity(Intent(candidate), 0) != null
        }
        return Intent(action).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
    }

    /**
     * Pure: returns the first action string in [SECURITY_ACTIONS] that
     * [canResolve] accepts, falling through to [Settings.ACTION_SETTINGS]
     * when none match. JVM-testable — no `Intent` construction.
     */
    internal fun pickSecurityAction(canResolve: (String) -> Boolean): String =
        SECURITY_ACTIONS.firstOrNull(canResolve) ?: Settings.ACTION_SETTINGS

    /**
     * `ACTION_VIEW` on the content URI of the exported .crt. Grants
     * read permission so the receiving app (typically Files) can open
     * the URI without owning the document. MIME explicitly set so
     * launchers don't fall back to a generic browser.
     */
    fun buildViewCertFileIntent(contentUri: Uri): Intent =
        Intent(Intent.ACTION_VIEW).apply {
            setDataAndType(contentUri, MIME_CA)
            addFlags(
                Intent.FLAG_GRANT_READ_URI_PERMISSION or
                    Intent.FLAG_ACTIVITY_NEW_TASK,
            )
        }

    /**
     * Which Settings IA should the on-screen step labels mirror? Read
     * `Build.MANUFACTURER` once per composition; Samsung devices route
     * through Biometrics & security → Other security settings →
     * Install from device storage. Stock AOSP and most other OEMs sit
     * the entry-point under Security & privacy → Encryption &
     * credentials.
     */
    fun detectFlavor(): SettingsFlavor =
        if (Build.MANUFACTURER.equals("samsung", ignoreCase = true)) {
            SettingsFlavor.SAMSUNG
        } else {
            SettingsFlavor.AOSP
        }

    private const val MIME_CA = "application/x-x509-ca-cert"
}

/**
 * Picks the string set rendered by the on-screen step list. The actual
 * navigation is identical on the two paths — only the menu labels
 * differ.
 */
enum class SettingsFlavor { SAMSUNG, AOSP }
