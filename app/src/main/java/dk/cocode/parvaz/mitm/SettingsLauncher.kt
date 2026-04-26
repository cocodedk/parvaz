package dk.cocode.parvaz.mitm

import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.provider.Settings
import dk.cocode.parvaz.R

/**
 * Builds the Intents handed off to system Settings from the CA install
 * screen. Pure functions — no Context, fakeable in unit tests by
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
        Settings.ACTION_PRIVACY_SETTINGS,
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
            setDataAndType(contentUri, CA_MIME_TYPE)
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
}

/**
 * Picks the string set rendered by the on-screen step list. The actual
 * navigation is identical on the two paths — only the menu labels
 * differ.
 */
enum class SettingsFlavor(val stepLabels: IntArray) {
    SAMSUNG(intArrayOf(
        R.string.ca_step_1_samsung,
        R.string.ca_step_2_samsung,
        R.string.ca_step_3_samsung,
        R.string.ca_step_4_samsung,
        R.string.ca_step_5_samsung,
    )),
    AOSP(intArrayOf(
        R.string.ca_step_1_aosp,
        R.string.ca_step_2_aosp,
        R.string.ca_step_3_aosp,
        R.string.ca_step_4_aosp,
        R.string.ca_step_5_aosp,
    )),
}
