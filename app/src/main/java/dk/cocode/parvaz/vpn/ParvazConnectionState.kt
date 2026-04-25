package dk.cocode.parvaz.vpn

/**
 * Lifecycle phase of the VPN connection, exported on
 * [ParvazVpnService.state] as a snapshot so the MainScreen can drive
 * the main-screen UI without binding to the service.
 */
enum class ConnectionState { DISCONNECTED, CONNECTING, CONNECTED, FAILED }

/**
 * Snapshot of the service's current session. [connectedAtMs] is set
 * only when [phase] == CONNECTED; the MainViewModel reads it to compute
 * uptime that survives activity recreation — re-reading the flow alone
 * would restart the counter from zero every time.
 */
data class SessionState(
    val phase: ConnectionState,
    val connectedAtMs: Long = 0L,
) {
    companion object {
        fun disconnected() = SessionState(ConnectionState.DISCONNECTED)
        fun connecting() = SessionState(ConnectionState.CONNECTING)
        fun connected(atMs: Long) = SessionState(ConnectionState.CONNECTED, atMs)
        fun failed() = SessionState(ConnectionState.FAILED)
    }
}
