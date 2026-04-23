package dk.cocode.parvaz.vpn

import android.content.Context
import android.util.Log
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.withContext
import kotlinx.coroutines.withTimeoutOrNull
import java.io.BufferedReader
import java.io.File
import java.io.InputStreamReader

/**
 * CoreLauncher spawns the Go sidecar (`libparvaz.so`) as a child process,
 * pipes a JSON config to its stdin, and waits for the single-line "READY"
 * handshake on stdout. Once READY, the sidecar is listening on
 * `127.0.0.1:<listenPort>` as a SOCKS5 server that the VpnService's
 * tun2socks bridge will point the TUN fd at.
 *
 * The binary lives in `nativeLibraryDir` because we ship the Go build
 * artifact as `jniLibs/<abi>/libparvaz.so`. Android's installer
 * extracts those to disk with execute permission (enabled by
 * `packaging.jniLibs.useLegacyPackaging = true` in app/build.gradle.kts).
 *
 * Not unit-testable on the JVM — `nativeLibraryDir` is Android-only and
 * the binary is built for Android. See
 * `app/src/androidTest/…/CoreLauncherInstrumentedTest.kt`.
 */
class CoreLauncher(
    context: Context,
    private val ioDispatcher: CoroutineDispatcher = Dispatchers.IO,
) {
    enum class State { IDLE, STARTING, READY, DEAD }

    private val appContext = context.applicationContext
    private val _state = MutableStateFlow(State.IDLE)
    val state: StateFlow<State> = _state

    private var process: Process? = null

    /**
     * Start the sidecar. Suspends until READY or until the READY handshake
     * times out (default 10s). Returns Result.failure with the reason on
     * any startup error.
     *
     * Only one sidecar per CoreLauncher; call [stop] before restarting.
     */
    suspend fun start(
        config: SidecarConfig,
        readyTimeoutMs: Long = 10_000L,
    ): Result<Unit> = withContext(ioDispatcher) {
        if (_state.value != State.IDLE && _state.value != State.DEAD) {
            return@withContext Result.failure(
                IllegalStateException("CoreLauncher not idle (state=${_state.value})"),
            )
        }
        _state.value = State.STARTING

        val binary = nativeBinary()
            ?: return@withContext fail("libparvaz.so not found in nativeLibraryDir")

        val pb = ProcessBuilder(binary.absolutePath, "-stdin")
            .redirectErrorStream(false)
        val proc = try {
            pb.start()
        } catch (e: Exception) {
            return@withContext fail("spawn failed: ${e.message}")
        }
        process = proc

        // Pipe the JSON config to stdin. The Go side reads until EOF on
        // stdin, so we must close it after writing or parvazd blocks.
        try {
            proc.outputStream.use { out ->
                out.write(config.toJson().toByteArray(Charsets.UTF_8))
                out.write('\n'.code)
            }
        } catch (e: Exception) {
            proc.destroy()
            return@withContext fail("write stdin: ${e.message}")
        }

        val reader = BufferedReader(InputStreamReader(proc.inputStream, Charsets.UTF_8))
        val line = withTimeoutOrNull(readyTimeoutMs) { reader.readLine() }
        when (line) {
            null -> {
                proc.destroy()
                return@withContext fail("READY handshake timed out (${readyTimeoutMs}ms)")
            }
            "READY" -> {
                _state.value = State.READY
                Log.i(TAG, "sidecar READY (pid=?)")
                Result.success(Unit)
            }
            else -> {
                proc.destroy()
                fail("expected READY, got: $line")
            }
        }
    }

    /** Tear down the sidecar. Safe to call from any state. */
    fun stop() {
        process?.destroy()
        process = null
        _state.value = State.IDLE
    }

    /** True if the underlying process is still alive. */
    fun isAlive(): Boolean = process?.isAlive == true

    private fun nativeBinary(): File? {
        val dir = appContext.applicationInfo.nativeLibraryDir ?: return null
        val f = File(dir, "libparvaz.so")
        return f.takeIf { it.isFile && it.canExecute() }
    }

    private fun fail(reason: String): Result<Unit> {
        _state.value = State.DEAD
        Log.e(TAG, "launcher: $reason")
        return Result.failure(IllegalStateException(reason))
    }

    private companion object {
        const val TAG = "CoreLauncher"
    }
}
