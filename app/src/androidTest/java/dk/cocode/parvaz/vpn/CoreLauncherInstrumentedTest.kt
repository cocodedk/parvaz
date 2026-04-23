package dk.cocode.parvaz.vpn

import android.content.Context
import android.os.Build
import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import dk.cocode.parvaz.settings.Access
import kotlinx.coroutines.runBlocking
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Assume.assumeTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import java.io.File
import java.net.InetSocketAddress
import java.net.Socket

/**
 * Spawns the real Go sidecar (`libparvaz.so`) on the emulator / device
 * and verifies the READY handshake completes and the SOCKS5 port is
 * bound. Traffic forwarding is a later milestone (M15b tun2socks); this
 * test just proves the process-launching plumbing.
 *
 * Requires libparvaz.so to be cross-compiled and packaged for the ABI
 * the emulator is running (typically arm64-v8a for the Medium_Phone
 * AVD — see CLAUDE.md §Build commands).
 */
@RunWith(AndroidJUnit4::class)
class CoreLauncherInstrumentedTest {
    private lateinit var context: Context
    private lateinit var launcher: CoreLauncher
    private lateinit var dataDir: File

    @Before
    fun setUp() {
        // libparvaz.so is built for android/arm64 only — android/amd64
        // needs cgo + NDK clang which we don't set up. Translation on
        // x86_64 emulators runs apps fine, but Android's native_bridge
        // does not extend to child processes spawned via ProcessBuilder,
        // so the Go binary segfaults on exec. Skip the test on non-arm64
        // environments; it runs green on any arm64 device/emulator.
        assumeTrue(
            "libparvaz.so is arm64-only; x86_64 translation does not reach ProcessBuilder children",
            Build.SUPPORTED_64_BIT_ABIS.any { it == "arm64-v8a" } && !isX86Host(),
        )
        context = ApplicationProvider.getApplicationContext()
        dataDir = File(context.filesDir, "sidecar-test").apply {
            deleteRecursively()
            mkdirs()
        }
        launcher = CoreLauncher(context)
    }

    private fun isX86Host(): Boolean =
        Build.SUPPORTED_ABIS.firstOrNull()?.startsWith("x86") == true

    @After
    fun tearDown() {
        if (::launcher.isInitialized) launcher.stop()
        if (::dataDir.isInitialized) dataDir.deleteRecursively()
    }

    @Test
    fun sidecarStartsAndReachesReady() = runBlocking {
        val cfg = SidecarConfig(
            access = Access("AKfycby-TEST", "test-key", "tester"),
            dataDir = dataDir.absolutePath,
            listenPort = FREE_PORT,
        )
        val result = launcher.start(cfg)
        assertTrue("start() failed: ${result.exceptionOrNull()}", result.isSuccess)
        assertEquals(CoreLauncher.State.READY, launcher.state.value)
        assertTrue("process should be alive after READY", launcher.isAlive())

        // Port should accept a TCP connection immediately after READY —
        // the sidecar's socks5.Server is listening before it prints READY.
        val sock = Socket()
        try {
            sock.connect(InetSocketAddress("127.0.0.1", FREE_PORT), 2_000)
            assertTrue("socket connected but not usable", sock.isConnected)
        } finally {
            sock.close()
        }

        // CA should have been generated under dataDir/ca/ (mitm.LoadOrCreate).
        val caCert = File(dataDir, "ca/ca.crt")
        assertTrue("CA cert not created at ${caCert.absolutePath}", caCert.isFile)
    }

    @Test
    fun startFailsGracefullyWhenPortAlreadyBound() = runBlocking {
        // Grab a port so the sidecar's Listen fails.
        val blocker = java.net.ServerSocket()
        blocker.reuseAddress = false
        blocker.bind(InetSocketAddress("127.0.0.1", FREE_PORT_ALT))
        try {
            val cfg = SidecarConfig(
                access = Access("AKfycby-TEST", "test-key", null),
                dataDir = dataDir.absolutePath,
                listenPort = FREE_PORT_ALT,
            )
            val result = launcher.start(cfg, readyTimeoutMs = 3_000L)
            // Either we time out waiting for READY, or parvazd exits on
            // bind failure and readLine returns non-READY. Both are
            // launcher-DEAD outcomes.
            assertTrue("expected failure, got success", result.isFailure)
            assertEquals(CoreLauncher.State.DEAD, launcher.state.value)
            assertNotNull(result.exceptionOrNull())
        } finally {
            blocker.close()
        }
    }

    @Test
    fun stopIsIdempotent() {
        launcher.stop() // from IDLE
        launcher.stop() // from IDLE (second call)
        // no crash is the assertion
        assertEquals(CoreLauncher.State.IDLE, launcher.state.value)
    }

    private companion object {
        // Use high ports to avoid clashing with anything real on the emulator.
        const val FREE_PORT = 28_010
        const val FREE_PORT_ALT = 28_011
    }
}
