package dk.cocode.parvaz.mitm

import android.content.Context
import android.os.Build
import android.provider.MediaStore
import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import kotlinx.coroutines.runBlocking
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith

/**
 * Validates [CaExporter] against a real Android runtime. The two
 * branches we care about:
 *
 *  • API 29+: MediaStore.Downloads insert with IS_PENDING=1 during
 *    write, cleared after — the file must be visible to the system
 *    picker afterwards (no `is_pending` filter returns it).
 *  • API 24-28: external app-files Download/ + FileProvider URI.
 *
 * On modern emulators (API 29+) only the first branch executes; the
 * legacy branch is exercised by lower-API test runs.
 */
@RunWith(AndroidJUnit4::class)
class CaExporterInstrumentedTest {

    private lateinit var context: Context
    private lateinit var exporter: CaExporter
    private val testFilename = "parvaz-ca-test.crt"

    @Before
    fun setUp() {
        context = ApplicationProvider.getApplicationContext()
        exporter = CaExporter(context)
    }

    @After
    fun cleanUp() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            val resolver = context.contentResolver
            resolver.delete(
                MediaStore.Downloads.EXTERNAL_CONTENT_URI,
                "${MediaStore.Downloads.DISPLAY_NAME} LIKE ?",
                arrayOf("parvaz-ca%.crt"),
            )
        }
    }

    @Test
    fun export_writesPemAndReturnsReadableUri() = runBlocking {
        val pem = "-----BEGIN CERTIFICATE-----\nDEADBEEF\n-----END CERTIFICATE-----\n"
            .toByteArray()
        val result = exporter.export(pem, testFilename)

        assertNotNull("exported URI", result.contentUri)
        assertTrue("display path mentions filename", result.displayPath.contains(testFilename))

        // Round-trip the bytes back through the URI to confirm the writer
        // actually emitted what we passed in.
        val readBack = context.contentResolver.openInputStream(result.contentUri)?.use { it.readBytes() }
            ?: error("openInputStream returned null for ${result.contentUri}")
        assertEquals(pem.size, readBack.size)
        assertEquals(pem.toList(), readBack.toList())
    }

    @Test
    fun export_clearsIsPendingSoFileIsVisibleToPicker() = runBlocking {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.Q) {
            // Legacy branch never sets IS_PENDING — no-op here.
            return@runBlocking
        }
        val pem = "-----BEGIN CERTIFICATE-----\nABCDEF\n-----END CERTIFICATE-----\n"
            .toByteArray()
        exporter.export(pem, testFilename)

        // Default query (no IS_PENDING filter) should see the file —
        // proves we cleared the pending flag after flush.
        val cursor = context.contentResolver.query(
            MediaStore.Downloads.EXTERNAL_CONTENT_URI,
            arrayOf(MediaStore.Downloads.DISPLAY_NAME, MediaStore.Downloads.MIME_TYPE),
            "${MediaStore.Downloads.DISPLAY_NAME} = ?",
            arrayOf(testFilename),
            null,
        ) ?: error("MediaStore query returned null cursor")
        cursor.use {
            assertTrue("file visible to picker after export", it.moveToFirst())
            val mime = it.getString(it.getColumnIndexOrThrow(MediaStore.Downloads.MIME_TYPE))
            assertEquals(CA_MIME_TYPE, mime)
        }
    }

    @Test
    fun export_isIdempotentAcrossRepeatedCalls() = runBlocking {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.Q) return@runBlocking
        val first = exporter.export("first".toByteArray(), testFilename)
        val second = exporter.export("second-call-replaces".toByteArray(), testFilename)
        // Different content URIs are fine (MediaStore can re-allocate);
        // what matters is exactly one row exists for the display name.
        val cursor = context.contentResolver.query(
            MediaStore.Downloads.EXTERNAL_CONTENT_URI,
            arrayOf(MediaStore.Downloads._ID),
            "${MediaStore.Downloads.DISPLAY_NAME} = ?",
            arrayOf(testFilename),
            null,
        ) ?: error("query returned null")
        cursor.use {
            assertEquals("export is idempotent — exactly one row per name", 1, it.count)
        }
        // Sanity: the most recent URI's contents reflect the second write.
        val readBack = context.contentResolver.openInputStream(second.contentUri)?.use { it.readBytes() }
        assertEquals("second-call-replaces", readBack?.decodeToString())
        // Don't reference `first` past write — MediaStore may have
        // recycled the URI; the row count assertion above is the
        // authoritative idempotency check.
        @Suppress("UNUSED_VARIABLE") val unused = first
    }

    @Test
    fun export_removesOlderParvazCaFilesFromDownloads() = runBlocking {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.Q) return@runBlocking

        exporter.export("old".toByteArray(), "parvaz-ca-old.crt")
        exporter.export("new".toByteArray(), "parvaz-ca-new.crt")

        val cursor = context.contentResolver.query(
            MediaStore.Downloads.EXTERNAL_CONTENT_URI,
            arrayOf(MediaStore.Downloads.DISPLAY_NAME),
            "${MediaStore.Downloads.DISPLAY_NAME} LIKE ?",
            arrayOf("parvaz-ca%.crt"),
            null,
        ) ?: error("query returned null")
        cursor.use {
            assertEquals("only the latest Parvaz CA export remains", 1, it.count)
            assertTrue(it.moveToFirst())
            val name = it.getString(it.getColumnIndexOrThrow(MediaStore.Downloads.DISPLAY_NAME))
            assertEquals("parvaz-ca-new.crt", name)
        }
    }
}
