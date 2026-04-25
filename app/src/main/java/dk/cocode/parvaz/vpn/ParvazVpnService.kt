package dk.cocode.parvaz.vpn

import android.app.Service
import android.content.Context
import android.content.Intent
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import android.net.VpnService
import android.os.Build
import android.os.ParcelFileDescriptor
import android.system.Os
import android.system.OsConstants
import android.util.Log
import dk.cocode.parvaz.settings.ParvazDataDir
import dk.cocode.parvaz.settings.ParvazSettings
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

/**
 * VpnService that establishes the TUN, spawns the Go sidecar (SOCKS5 on
 * 127.0.0.1:<listenPort>), hands the TUN fd over, and starts tun2socks.
 * Companion-object [state] StateFlow drives the MainScreen UI — single
 * source of truth, survives activity recreation but not process death.
 */
class ParvazVpnService : VpnService() {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private var tun: ParcelFileDescriptor? = null
    private var launcher: CoreLauncher? = null
    private var startJob: Job? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_START -> {
                // Foreground promotion is required within 5s of a
                // startForegroundService() call on API 26+. Promote
                // before scheduling any real work.
                startForeground(
                    NOTIFICATION_ID,
                    VpnNotification.build(this, ongoing = true),
                )
                scheduleStart()
            }
            ACTION_STOP -> teardown()
            else -> Log.w(TAG, "unknown action: ${intent?.action}")
        }
        return Service.START_NOT_STICKY
    }

    override fun onDestroy() {
        // Physical resources only — _state is owned by the start/stop
        // paths so FAILED isn't clobbered on service destruction after
        // a failed start.
        cleanup()
        scope.cancel()
        super.onDestroy()
    }

    private fun scheduleStart() {
        startJob?.cancel()
        _state.value = SessionState.connecting()
        startJob = scope.launch {
            // tun2socks needs FD_CLOEXEC-clear via Os.fcntlInt (API 30+).
            // Pre-R: TUN installs but every packet blackholes — fail fast.
            if (Build.VERSION.SDK_INT < Build.VERSION_CODES.R) {
                Log.e(TAG, "Android ${Build.VERSION.SDK_INT} < R; tun2socks fd passing unavailable")
                fail(FailReason.UNKNOWN)
                return@launch
            }
            val access = ParvazSettings(this@ParvazVpnService).load()
            if (access == null) {
                Log.e(TAG, "no Access saved — aborting VPN start")
                fail(FailReason.NO_ACCESS)
                return@launch
            }

            if (!hasInternet()) {
                Log.w(TAG, "no active network before tun establish — aborting")
                fail(FailReason.NO_INTERNET)
                return@launch
            }

            // Exempt our own package so parvazd's outbound (Google edge
            // fronter, Apps Script POSTs) bypasses our TUN — otherwise
            // the sidecar's sockets would loop back into tun2socks.
            val builder = Builder()
                .setSession(SESSION_NAME)
                .addAddress(TUN_ADDRESS, TUN_PREFIX)
                .addRoute("0.0.0.0", 0)
                .addDnsServer(DNS_SERVER)
                .setMtu(TUN_MTU)
            try {
                builder.addDisallowedApplication(packageName)
            } catch (e: Exception) {
                Log.w(TAG, "addDisallowedApplication(${packageName}) failed: ${e.message}")
            }
            tun = builder.establish()

            if (tun == null) {
                Log.e(TAG, "establish() returned null — VPN permission revoked?")
                fail(FailReason.VPN_REVOKED)
                return@launch
            }

            // Clear FD_CLOEXEC so ProcessBuilder's exec preserves the fd
            // on the child — without this the sidecar's os.NewFile gets a
            // closed fd and tun2socks reads EBADF. API ≥ 30 enforced above.
            val tunFd: Int = try {
                Os.fcntlInt(tun!!.fileDescriptor, OsConstants.F_SETFD, 0)
                tun!!.fd
            } catch (e: Throwable) {
                Log.e(TAG, "fcntl F_SETFD clear failed: ${e.message}")
                fail(FailReason.SIDECAR_FAILED)
                return@launch
            }

            // MUST point at the same dir CaInstallController uses or the
            // sidecar signs leaves with a CA the user never installed.
            val dataDir = ParvazDataDir.forContext(this@ParvazVpnService)
            val cfg = SidecarConfig(
                access = access,
                dataDir = dataDir.absolutePath,
                tunFD = tunFd,
                tunMTU = TUN_MTU,
            )

            launcher = CoreLauncher(this@ParvazVpnService).also { l ->
                val r = l.start(cfg)
                if (r.isFailure) {
                    Log.e(TAG, "sidecar failed: ${r.exceptionOrNull()}")
                    fail(FailReason.SIDECAR_FAILED)
                    return@launch
                }
            }
            _state.value = SessionState.connected(System.currentTimeMillis())
        }
    }

    // VALIDATED catches captive portals / downed upstream where the
    // transport still advertises INTERNET.
    private fun hasInternet(): Boolean {
        val cm = getSystemService(Context.CONNECTIVITY_SERVICE) as? ConnectivityManager
            ?: return true
        val caps = cm.getNetworkCapabilities(cm.activeNetwork ?: return false) ?: return false
        return caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET) &&
            caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_VALIDATED)
    }

    private fun cleanup() {
        launcher?.stop()
        launcher = null
        tun?.close()
        tun = null
    }

    private fun fail(reason: FailReason) {
        startJob?.cancel()
        startJob = null
        cleanup()
        _state.value = SessionState.failed(reason)
        stopSelf()
    }

    private fun teardown() {
        // Cancel any in-flight scheduleStart coroutine first — without
        // this, a user who resets access mid-CONNECTING could see the
        // startJob later write CONNECTED/FAILED after teardown cleared
        // DISCONNECTED, stranding them on onboarding with a running VPN.
        startJob?.cancel()
        startJob = null
        cleanup()
        _state.value = SessionState.disconnected()
        stopSelf()
    }

    companion object {
        const val ACTION_START = "dk.cocode.parvaz.vpn.START"
        const val ACTION_STOP = "dk.cocode.parvaz.vpn.STOP"

        private val _state = MutableStateFlow(SessionState.disconnected())
        /** Observe to drive the main-screen UI. See class-level doc for lifetime caveats. */
        val state: StateFlow<SessionState> = _state.asStateFlow()

        private const val NOTIFICATION_ID = 1
        private const val TAG = "ParvazVpnService"
        private const val SESSION_NAME = "Parvaz"
        private const val TUN_ADDRESS = "10.0.0.1"
        private const val TUN_PREFIX = 24
        private const val TUN_MTU = 1500
        // Synthetic in-TUN DNS target. Android UDP/53 → tun2socks → SOCKS5
        // UDP ASSOCIATE → parvazd DoH. Not a real server.
        private const val DNS_SERVER = "10.0.0.2"
    }
}
