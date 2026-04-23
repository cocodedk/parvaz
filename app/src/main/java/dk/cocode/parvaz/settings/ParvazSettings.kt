package dk.cocode.parvaz.settings

import android.content.Context
import android.content.SharedPreferences
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey

/**
 * ParvazSettings is the persistence layer for the one piece of
 * configuration a user ever enters (an [Access]) plus the preferred
 * display language.
 *
 * The **access key** is stored in `EncryptedSharedPreferences` — it's the
 * only piece of data whose leak would let a third party impersonate the
 * user against the Apps Script deployment. The encryption key lives in
 * the Android Keystore, hardware-backed where available.
 *
 * Everything else (deployment ID, display name, language) is not secret —
 * knowing a deployment ID alone does not authorize access. Stored in a
 * plain `SharedPreferences` file to keep the threat model honest.
 *
 * Default language is `fa` (Persian) because Parvaz is Farsi-first.
 *
 * This class cannot be unit-tested on the JVM — EncryptedSharedPreferences
 * depends on a real Android Keystore. See
 * `app/src/androidTest/…/ParvazSettingsInstrumentedTest.kt`.
 */
class ParvazSettings(context: Context) {
    private val appContext = context.applicationContext
    private val plain: SharedPreferences =
        appContext.getSharedPreferences(PLAIN_FILE, Context.MODE_PRIVATE)
    private val secure: SharedPreferences = EncryptedSharedPreferences.create(
        appContext,
        SECURE_FILE,
        MasterKey.Builder(appContext)
            .setKeyScheme(MasterKey.KeyScheme.AES256_GCM)
            .build(),
        EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
        EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
    )

    /** Persists access. Overwrites any existing value. */
    fun save(access: Access) {
        secure.edit().putString(KEY_ACCESS_KEY, access.accessKey).apply()
        plain.edit()
            .putString(KEY_DEPLOYMENT_ID, access.deploymentId)
            .also { editor ->
                if (access.displayName != null) {
                    editor.putString(KEY_DISPLAY_NAME, access.displayName)
                } else {
                    editor.remove(KEY_DISPLAY_NAME)
                }
            }
            .apply()
    }

    /** Returns the saved access, or null if the user hasn't imported one yet. */
    fun load(): Access? {
        val accessKey = secure.getString(KEY_ACCESS_KEY, null) ?: return null
        val deploymentId = plain.getString(KEY_DEPLOYMENT_ID, null) ?: return null
        val displayName = plain.getString(KEY_DISPLAY_NAME, null)
        return Access(
            deploymentId = deploymentId,
            accessKey = accessKey,
            displayName = displayName,
        )
    }

    /** Wipes the saved access; used by the "reset" action in settings. */
    fun clearAccess() {
        secure.edit().remove(KEY_ACCESS_KEY).apply()
        plain.edit()
            .remove(KEY_DEPLOYMENT_ID)
            .remove(KEY_DISPLAY_NAME)
            .apply()
    }

    /** UI language: "fa" (default) or "en". Mutable. */
    var language: String
        get() = plain.getString(KEY_LANGUAGE, DEFAULT_LANGUAGE) ?: DEFAULT_LANGUAGE
        set(value) {
            plain.edit().putString(KEY_LANGUAGE, value).apply()
        }

    /**
     * True once the user has completed every onboarding step (import,
     * CA install, VPN permission). Gates the switch from OnboardingHost
     * to MainScreen — without it, MainActivity would jump to main as
     * soon as Access persists, skipping CA_INSTALL and VPN_EXPLAIN on
     * any activity recreation.
     */
    var isOnboardingComplete: Boolean
        get() = plain.getBoolean(KEY_ONBOARDING_COMPLETE, false)
        set(value) {
            plain.edit().putBoolean(KEY_ONBOARDING_COMPLETE, value).apply()
        }

    private companion object {
        const val PLAIN_FILE = "parvaz_prefs"
        const val SECURE_FILE = "parvaz_secure"

        const val KEY_ACCESS_KEY = "access_key"
        const val KEY_DEPLOYMENT_ID = "deployment_id"
        const val KEY_DISPLAY_NAME = "display_name"
        const val KEY_LANGUAGE = "language"
        const val KEY_ONBOARDING_COMPLETE = "onboarding_complete"

        const val DEFAULT_LANGUAGE = "fa"
    }
}
