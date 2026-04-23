package dk.cocode.parvaz.ui.main

import android.app.Application
import android.content.Intent
import androidx.core.content.ContextCompat
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import dk.cocode.parvaz.vpn.ConnectionState
import dk.cocode.parvaz.vpn.ParvazVpnService
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.launch

data class MainUiState(
    val phase: ConnectionState = ConnectionState.DISCONNECTED,
    val uptimeSeconds: Long = 0L,
)

/**
 * Drives the M13 main screen. Observes [ParvazVpnService.state] (the
 * companion StateFlow on the service), surfaces a [MainUiState], and
 * owns an uptime ticker that runs only while CONNECTED.
 *
 * Uptime is derived from the service's own `connectedAtMs` timestamp,
 * not from the moment the VM started observing — that way activity
 * recreation replays the real session age instead of resetting the
 * counter to zero.
 *
 * Scoped to the Activity — `by viewModels()` — so configuration changes
 * don't reset the ticker or the collected state.
 */
class MainViewModel(app: Application) : AndroidViewModel(app) {
    private val _ui = MutableStateFlow(MainUiState())
    val ui: StateFlow<MainUiState> = _ui.asStateFlow()

    private var tickerJob: Job? = null
    private var activeConnectedAtMs: Long = 0L

    init {
        viewModelScope.launch {
            ParvazVpnService.state.collectLatest { s ->
                _ui.value = _ui.value.copy(phase = s.phase)
                when (s.phase) {
                    ConnectionState.CONNECTED -> startTicker(s.connectedAtMs)
                    else -> stopTicker()
                }
            }
        }
    }

    fun connect() {
        dispatch(ParvazVpnService.ACTION_START)
    }

    fun disconnect() {
        dispatch(ParvazVpnService.ACTION_STOP)
    }

    private fun dispatch(action: String) {
        val ctx = getApplication<Application>()
        val intent = Intent(ctx, ParvazVpnService::class.java).setAction(action)
        if (action == ParvazVpnService.ACTION_START) {
            ContextCompat.startForegroundService(ctx, intent)
        } else {
            ctx.startService(intent)
        }
    }

    private fun startTicker(connectedAtMs: Long) {
        // If already ticking for the same session, let it keep running.
        if (tickerJob?.isActive == true && activeConnectedAtMs == connectedAtMs) return
        stopTicker()
        activeConnectedAtMs = connectedAtMs
        tickerJob = viewModelScope.launch {
            while (true) {
                val elapsed = (System.currentTimeMillis() - connectedAtMs) / 1000
                _ui.value = _ui.value.copy(uptimeSeconds = elapsed)
                delay(1_000)
            }
        }
    }

    private fun stopTicker() {
        tickerJob?.cancel()
        tickerJob = null
        activeConnectedAtMs = 0L
        _ui.value = _ui.value.copy(uptimeSeconds = 0L)
    }
}
