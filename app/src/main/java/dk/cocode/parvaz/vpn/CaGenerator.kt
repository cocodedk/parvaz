package dk.cocode.parvaz.vpn

import android.content.Context
import android.util.Log
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.runInterruptible
import kotlinx.coroutines.withContext
import kotlinx.coroutines.withTimeoutOrNull
import java.io.File

/**
 * CaGenerator runs `libparvaz.so -gen-ca -data-dir <path>` as a one-shot
 * child process, waits for exit 0, and returns the PEM bytes the
 * CaInstallScreen will hand to `ACTION_MANAGE_CA_CERTIFICATES`.
 *
 * Idempotent on the Go side (mitm.LoadOrCreate) — safe to call every
 * time the user opens the CA install step.
 *
 * Not unit-testable on the JVM: `nativeLibraryDir` is Android-only and
 * `libparvaz.so` is cross-compiled for android/arm64.
 */
class CaGenerator(
    context: Context,
    private val ioDispatcher: CoroutineDispatcher = Dispatchers.IO,
) {
    private val appContext = context.applicationContext

    /**
     * Materialises the CA under `<dataDir>/ca/` and reads back `ca.crt`.
     * Total runtime is a few milliseconds under normal conditions — the
     * timeout is a safety net for a hung process, not a steady-state.
     */
    suspend fun generate(dataDir: File, timeoutMs: Long = 10_000L): Result<ByteArray> =
        withContext(ioDispatcher) {
            val binary = nativeBinary()
                ?: return@withContext Result.failure(
                    IllegalStateException("libparvaz.so not found in nativeLibraryDir"),
                )
            dataDir.mkdirs()

            val pb = ProcessBuilder(
                binary.absolutePath,
                "-gen-ca",
                "-data-dir", dataDir.absolutePath,
            ).redirectErrorStream(true)

            val proc = try {
                pb.start()
            } catch (e: Exception) {
                return@withContext Result.failure(
                    IllegalStateException("spawn parvazd -gen-ca failed: ${e.message}", e),
                )
            }

            val exit = withTimeoutOrNull(timeoutMs) {
                runInterruptible { proc.waitFor() }
            }
            if (exit == null) {
                proc.destroy()
                return@withContext Result.failure(
                    IllegalStateException("parvazd -gen-ca timed out after ${timeoutMs}ms"),
                )
            }
            if (exit != 0) {
                val err = proc.inputStream.bufferedReader().use { it.readText().trim() }
                Log.e(TAG, "parvazd -gen-ca exit=$exit: $err")
                return@withContext Result.failure(
                    IllegalStateException("parvazd -gen-ca exit=$exit"),
                )
            }

            val caCert = File(dataDir, "ca/${CA_FILENAME}")
            if (!caCert.isFile) {
                return@withContext Result.failure(
                    IllegalStateException("ca.crt not written at ${caCert.absolutePath}"),
                )
            }
            Result.success(caCert.readBytes())
        }

    private fun nativeBinary(): File? {
        val dir = appContext.applicationInfo.nativeLibraryDir ?: return null
        val f = File(dir, "libparvaz.so")
        return f.takeIf { it.isFile && it.canExecute() }
    }

    companion object {
        private const val TAG = "CaGenerator"
        private const val CA_FILENAME = "ca.crt"
    }
}
