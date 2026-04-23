package dk.cocode.parvaz.ui.main

import android.app.Application
import android.content.Context
import android.content.Intent
import androidx.core.content.ContextCompat
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import dk.cocode.parvaz.vpn.ParvazVpnService
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.launch

data class MainUiState(
    val phase: ParvazVpnService.ConnectionState = ParvazVpnService.ConnectionState.DISCONNECTED,
    val uptimeSeconds: Long = 0L,
)

/**
 * Drives the M13 main screen. Observes [ParvazVpnService.state] (the
 * companion StateFlow on the service), surfaces a [MainUiState], and
 * owns an uptime ticker that runs only while CONNECTED.
 *
 * Scoped to the Activity — `by viewModels()` — so configuration changes
 * don't reset the ticker or the collected state.
 */
class MainViewModel(app: Application) : AndroidViewModel(app) {
    private val _ui = MutableStateFlow(MainUiState())
    val ui: StateFlow<MainUiState> = _ui.asStateFlow()

    private var tickerJob: Job? = null
    private var connectedAtMs: Long = 0L

    init {
        viewModelScope.launch {
            ParvazVpnService.state.collectLatest { s ->
                _ui.value = _ui.value.copy(phase = s)
                when (s) {
                    ParvazVpnService.ConnectionState.CONNECTED -> startTicker()
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

    private fun startTicker() {
        if (tickerJob?.isActive == true) return
        connectedAtMs = System.currentTimeMillis()
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
        _ui.value = _ui.value.copy(uptimeSeconds = 0L)
    }
}

/** Convenience for tests / previews. Unused by the app runtime. */
internal fun mainContextIntent(ctx: Context, action: String): Intent =
    Intent(ctx, ParvazVpnService::class.java).setAction(action)
