package dk.cocode.parvaz.update

/**
 * State machine for the in-app updater. Lives in a Composable's
 * `rememberSaveable` while the settings sheet is open — process death
 * resets to [Idle] which is fine for a manual flow.
 */
sealed interface UpdateState {
    data object Idle : UpdateState
    data object Checking : UpdateState
    data object UpToDate : UpdateState
    data class Available(val release: ReleaseInfo) : UpdateState
    data class Disconnecting(val release: ReleaseInfo) : UpdateState
    data class Downloading(
        val release: ReleaseInfo,
        val downloadedBytes: Long,
        val totalBytes: Long,
    ) : UpdateState
    data class InstallerHandoff(val release: ReleaseInfo) : UpdateState
    /** Carries the [ReleaseInfo] so the user's retry tap (after they
     *  flip "install unknown apps" in system settings) can resume the
     *  download without going back through `check()`. */
    data class NeedsUnknownSources(val release: ReleaseInfo) : UpdateState
    sealed interface Failure : UpdateState {
        data object Network : Failure
        data object NoAsset : Failure
        data object ShaMismatch : Failure
    }
}
