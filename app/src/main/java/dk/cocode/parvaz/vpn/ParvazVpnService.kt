package dk.cocode.parvaz.vpn

import android.app.Service
import android.content.Intent
import android.net.VpnService
import android.os.ParcelFileDescriptor
import android.util.Log
import dk.cocode.parvaz.settings.ParvazSettings
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch
import java.io.File

/**
 * ParvazVpnService establishes a system VPN TUN interface and spawns
 * the Go sidecar that will serve SOCKS5 on `127.0.0.1:<listenPort>`.
 *
 * This milestone (M15a) stops at "sidecar is running, TUN is up". The
 * bridge that pipes packets between the TUN file descriptor and the
 * SOCKS5 port lives in M15b (tun2socks). Until then, traffic routed
 * through the VPN has nowhere to go and will drop.
 *
 * Start with an explicit action + stop with another — standard
 * VpnService pattern so the app UI stays the only place that initiates
 * connection state changes.
 */
class ParvazVpnService : VpnService() {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private var tun: ParcelFileDescriptor? = null
    private var launcher: CoreLauncher? = null
    private var startJob: Job? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_START -> scheduleStart()
            ACTION_STOP -> teardown()
            else -> Log.w(TAG, "unknown action: ${intent?.action}")
        }
        return Service.START_NOT_STICKY
    }

    override fun onDestroy() {
        teardown()
        scope.cancel()
        super.onDestroy()
    }

    private fun scheduleStart() {
        startJob?.cancel()
        startJob = scope.launch {
            val access = ParvazSettings(this@ParvazVpnService).load()
            if (access == null) {
                Log.e(TAG, "no Access saved — aborting VPN start")
                stopSelf()
                return@launch
            }

            // Establish TUN interface. The address / route here match
            // what the mhrv-rs Android port uses; tun2socks (M15b) will
            // own the FD once wired.
            tun = Builder()
                .setSession(SESSION_NAME)
                .addAddress(TUN_ADDRESS, TUN_PREFIX)
                .addRoute("0.0.0.0", 0)
                .addDnsServer(DNS_SERVER)
                .setMtu(TUN_MTU)
                .establish()

            if (tun == null) {
                Log.e(TAG, "establish() returned null — VPN permission revoked?")
                stopSelf()
                return@launch
            }

            val dataDir = File(filesDir, "sidecar").apply { mkdirs() }
            val cfg = SidecarConfig(access = access, dataDir = dataDir.absolutePath)

            launcher = CoreLauncher(this@ParvazVpnService).also { l ->
                val r = l.start(cfg)
                if (r.isFailure) {
                    Log.e(TAG, "sidecar failed: ${r.exceptionOrNull()}")
                    teardown()
                }
            }
        }
    }

    private fun teardown() {
        launcher?.stop()
        launcher = null
        tun?.close()
        tun = null
        stopSelf()
    }

    companion object {
        const val ACTION_START = "dk.cocode.parvaz.vpn.START"
        const val ACTION_STOP = "dk.cocode.parvaz.vpn.STOP"

        private const val TAG = "ParvazVpnService"
        private const val SESSION_NAME = "Parvaz"
        private const val TUN_ADDRESS = "10.0.0.1"
        private const val TUN_PREFIX = 24
        private const val TUN_MTU = 1500
        private const val DNS_SERVER = "8.8.8.8"
    }
}
