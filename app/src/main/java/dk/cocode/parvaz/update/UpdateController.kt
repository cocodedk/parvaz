package dk.cocode.parvaz.update

import android.app.Application
import android.content.Context
import android.net.ConnectivityManager
import android.os.Build
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import dk.cocode.parvaz.vpn.ConnectionState
import dk.cocode.parvaz.vpn.ParvazVpnService
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

/**
 * Drives the update state machine. Lives at the activity level via
 * `by viewModels()` so check + download survive configuration change.
 *
 * Talks to:
 *   - [GitHubReleasesClient] for the manifest
 *   - [ApkDownloader]        for streaming + sha256 verification
 *   - [ApkInstaller]         for the system handoff
 *   - [ParvazVpnService]     to disconnect before download (so the
 *                            github.com fetch bypasses our own MITM)
 *
 * Network bypass: when the VPN is active, we issue ACTION_STOP on
 * ParvazVpnService and wait until [ParvazVpnService.state] reports
 * DISCONNECTED before dialing GitHub. We additionally request a
 * non-VPN network via ConnectivityManager.bindProcessToNetwork on
 * Lollipop+; if that returns null we fall back to the default route
 * (acceptable since the VPN is already torn down).
 */
class UpdateController(app: Application) : AndroidViewModel(app) {

    private val client = GitHubReleasesClient()
    private val downloader = ApkDownloader()
    private val installer = ApkInstaller(app.applicationContext)

    private val _state = MutableStateFlow<UpdateState>(UpdateState.Idle)
    val state: StateFlow<UpdateState> = _state.asStateFlow()

    private var job: Job? = null

    fun check(currentVersion: Version) {
        if (_state.value is UpdateState.Downloading || _state.value is UpdateState.Disconnecting) return
        job?.cancel()
        job = viewModelScope.launch {
            _state.value = UpdateState.Checking
            val result = withContext(Dispatchers.IO) { client.fetchLatest() }
            _state.value = when (result) {
                is FetchResult.Success ->
                    if (result.release.version.isNewerThan(currentVersion))
                        UpdateState.Available(result.release)
                    else UpdateState.UpToDate
                FetchResult.NoAsset -> UpdateState.Failure.NoAsset
                FetchResult.NetworkError, FetchResult.Malformed -> UpdateState.Failure.Network
            }
        }
    }

    fun install(stopVpn: () -> Unit) {
        val release = when (val current = _state.value) {
            is UpdateState.Available -> current.release
            // User just flipped "install unknown apps" — resume with
            // the same release we already downloaded.
            is UpdateState.NeedsUnknownSources -> current.release
            else -> return
        }

        job?.cancel()
        job = viewModelScope.launch {
            try {
                _state.value = UpdateState.Disconnecting(release)
                withContext(Dispatchers.Main) { stopVpn() }
                waitForVpnDisconnected()
                bindProcessToNonVpn()

                _state.value = UpdateState.Downloading(release, 0L, release.apkSizeBytes)
                val dest = installer.destinationFile()
                val outcome = withContext(Dispatchers.IO) {
                    downloader.download(
                        apkUrl = release.apkUrl,
                        sha256Url = release.sha256Url,
                        destination = dest,
                        totalBytes = release.apkSizeBytes,
                        onProgress = { downloaded, total ->
                            _state.value = UpdateState.Downloading(release, downloaded, total)
                        },
                    )
                }
                _state.value = when (outcome) {
                    is ApkDownloadOutcome.Success -> {
                        when (installer.install(outcome.file)) {
                            // System installer takes the screen; on success
                            // the OS replaces this process so unbinding here
                            // is moot but harmless.
                            InstallOutcome.Launched -> UpdateState.InstallerHandoff(release)
                            InstallOutcome.NeedsUnknownSourcesPermission -> UpdateState.NeedsUnknownSources(release)
                        }
                    }
                    ApkDownloadOutcome.ShaMismatch -> UpdateState.Failure.ShaMismatch
                    is ApkDownloadOutcome.NetworkError -> UpdateState.Failure.Network
                }
            } finally {
                // Privacy/VPN-app correctness: any non-Launched terminal
                // state, *and* coroutine cancellation, must release the
                // process-wide non-VPN binding. Otherwise a user who
                // re-enables Parvaz after a failed update would silently
                // bypass the tunnel until app process death.
                if (_state.value !is UpdateState.InstallerHandoff) {
                    unbindProcessNetwork()
                }
            }
        }
    }

    private suspend fun waitForVpnDisconnected() {
        // Best-effort: if the service was never running, state is already
        // DISCONNECTED. Cap wait at 4s so a stuck service doesn't wedge
        // the updater forever.
        val deadline = System.currentTimeMillis() + 4_000
        while (System.currentTimeMillis() < deadline) {
            if (ParvazVpnService.state.value.phase == ConnectionState.DISCONNECTED) return
            delay(150)
        }
    }

    private fun bindProcessToNonVpn() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.M) return
        val cm = connectivityManager() ?: return
        val networks = cm.allNetworks
        val nonVpn = networks.firstOrNull { net ->
            val caps = cm.getNetworkCapabilities(net) ?: return@firstOrNull false
            !caps.hasTransport(android.net.NetworkCapabilities.TRANSPORT_VPN) &&
                caps.hasCapability(android.net.NetworkCapabilities.NET_CAPABILITY_INTERNET)
        }
        if (nonVpn != null) cm.bindProcessToNetwork(nonVpn)
    }

    private fun unbindProcessNetwork() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.M) return
        connectivityManager()?.bindProcessToNetwork(null)
    }

    private fun connectivityManager(): ConnectivityManager? =
        getApplication<Application>()
            .getSystemService(Context.CONNECTIVITY_SERVICE) as? ConnectivityManager
}
