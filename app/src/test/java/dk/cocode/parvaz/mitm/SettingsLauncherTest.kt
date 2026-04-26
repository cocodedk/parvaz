package dk.cocode.parvaz.mitm

import android.provider.Settings
import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * Pure-JVM coverage of the Settings-action fallback chain. The full
 * Intent construction lives in [SettingsLauncher.buildSecurityIntent],
 * which needs a real PackageManager — those paths are exercised by
 * the instrumented onboarding test. Here we only verify the chain
 * picks the right action string given a fake resolver predicate.
 */
class SettingsLauncherTest {

    @Test
    fun pickSecurityAction_picksFirstResolverHit() {
        // Resolver answers for everything → the first listed action wins.
        val action = SettingsLauncher.pickSecurityAction { _ -> true }
        assertEquals(Settings.ACTION_SECURITY_SETTINGS, action)
    }

    @Test
    fun pickSecurityAction_fallsThroughToPrivacyOnPartialMatch() {
        // Stock AOSP exposes ACTION_SECURITY_SETTINGS, but pretend it
        // doesn't on this device. Privacy resolver answers next.
        val action = SettingsLauncher.pickSecurityAction { candidate ->
            candidate != Settings.ACTION_SECURITY_SETTINGS
        }
        assertEquals("android.settings.PRIVACY_SETTINGS", action)
    }

    @Test
    fun pickSecurityAction_fallsAllTheWayToTopLevelSettings() {
        // Resolver rejects everything. We still return *something*
        // launchable — top-level Settings is the universal fallback.
        val action = SettingsLauncher.pickSecurityAction { _ -> false }
        assertEquals(Settings.ACTION_SETTINGS, action)
    }

    @Test
    fun pickSecurityAction_doesNotCallResolverAfterMatch() {
        // Once a candidate matches, later candidates must not be probed
        // — keeps the chain cheap and avoids wasted resolveActivity hits.
        val seen = mutableListOf<String>()
        SettingsLauncher.pickSecurityAction { candidate ->
            seen += candidate
            candidate == Settings.ACTION_SECURITY_SETTINGS
        }
        assertEquals(listOf(Settings.ACTION_SECURITY_SETTINGS), seen)
    }
}
