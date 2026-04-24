package dk.cocode.parvaz.vpn

/**
 * Lifecycle phase of the VPN connection, exported on
 * [ParvazVpnService.state] as a snapshot so the MainScreen can drive
 * the main-screen UI without binding to the service.
 */
enum class ConnectionState { DISCONNECTED, CONNECTING, CONNECTED, FAILED }

/** Why the last connect attempt failed; main screen renders matching copy. */
enum class FailReason { NO_INTERNET, NO_ACCESS, VPN_REVOKED, SIDECAR_FAILED, UNKNOWN }

/**
 * Snapshot of the service's current session. [connectedAtMs] is set
 * only when [phase] == CONNECTED; the MainViewModel reads it to compute
 * uptime that survives activity recreation. [failReason] is non-null
 * only when [phase] == FAILED.
 */
data class SessionState(
    val phase: ConnectionState,
    val connectedAtMs: Long = 0L,
    val failReason: FailReason? = null,
) {
    companion object {
        fun disconnected() = SessionState(ConnectionState.DISCONNECTED)
        fun connecting() = SessionState(ConnectionState.CONNECTING)
        fun connected(atMs: Long) = SessionState(ConnectionState.CONNECTED, atMs)
        fun failed(reason: FailReason = FailReason.UNKNOWN) =
            SessionState(ConnectionState.FAILED, failReason = reason)
    }
}
