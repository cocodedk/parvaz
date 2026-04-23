package dk.cocode.parvaz.settings

import android.content.Context
import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class ParvazSettingsInstrumentedTest {
    private lateinit var context: Context
    private lateinit var settings: ParvazSettings

    @Before
    fun setUp() {
        context = ApplicationProvider.getApplicationContext()
        // Wipe any prior state. deleteSharedPreferences also works on the
        // EncryptedSharedPreferences file — it's still a SharedPreferences
        // file on disk, encryption notwithstanding.
        context.deleteSharedPreferences(PLAIN_FILE)
        context.deleteSharedPreferences(SECURE_FILE)
        settings = ParvazSettings(context)
    }

    @After
    fun tearDown() {
        context.deleteSharedPreferences(PLAIN_FILE)
        context.deleteSharedPreferences(SECURE_FILE)
    }

    @Test
    fun loadReturnsNullBeforeAnySave() {
        assertNull(settings.load())
    }

    @Test
    fun saveLoadRoundTrip_withDisplayName() {
        val original = Access("DEP123", "KEY456", "My Phone")
        settings.save(original)

        // Rebuild the instance to prove the data persists across process-
        // lifetime boundaries (EncryptedSharedPreferences reads from disk).
        val reloaded = ParvazSettings(context).load()
        assertEquals(original, reloaded)
    }

    @Test
    fun saveLoadRoundTrip_nullDisplayName() {
        val original = Access("DEP123", "KEY456", null)
        settings.save(original)
        assertEquals(original, ParvazSettings(context).load())
    }

    @Test
    fun saveOverwritesDisplayName() {
        settings.save(Access("DEP", "KEY", "First"))
        settings.save(Access("DEP", "KEY", "Second"))
        assertEquals("Second", settings.load()?.displayName)
    }

    @Test
    fun saveThenSaveWithoutDisplayName_clearsOldDisplayName() {
        settings.save(Access("DEP", "KEY", "Named"))
        settings.save(Access("DEP", "KEY", null))
        assertNull(settings.load()?.displayName)
    }

    @Test
    fun clearAccess_removesAll() {
        settings.save(Access("DEP", "KEY", "Named"))
        settings.clearAccess()
        assertNull(settings.load())
    }

    @Test
    fun languageDefaultsToFarsi() {
        assertEquals("fa", settings.language)
    }

    @Test
    fun language_persistsAcrossInstances() {
        settings.language = "en"
        assertEquals("en", ParvazSettings(context).language)
    }

    @Test
    fun accessKey_isNotReadableFromPlainPrefs() {
        // Sanity check on the threat model: the access key lives in the
        // encrypted file, not in the plain prefs. Reading the plain file
        // directly should never expose it.
        settings.save(Access("DEP", "SECRET-KEY", null))
        val plain = context.getSharedPreferences(PLAIN_FILE, Context.MODE_PRIVATE)
        val plainValues = plain.all.values.map { it?.toString().orEmpty() }
        for (v in plainValues) {
            if (v.contains("SECRET-KEY")) {
                throw AssertionError("access key leaked to plain prefs: $v")
            }
        }
    }

    private companion object {
        const val PLAIN_FILE = "parvaz_prefs"
        const val SECURE_FILE = "parvaz_secure"
    }
}
