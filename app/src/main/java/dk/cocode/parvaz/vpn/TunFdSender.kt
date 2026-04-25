package dk.cocode.parvaz.vpn

import android.net.LocalSocket
import android.net.LocalSocketAddress
import android.util.Log
import kotlinx.coroutines.delay
import java.io.FileDescriptor

/**
 * Sends an open file descriptor to parvazd over the abstract-namespace
 * UNIX socket the sidecar listens on. Required because Android's
 * ProcessBuilder.start() closes inherited fds ≥3 in the child even when
 * FD_CLOEXEC has been cleared, so the TUN fd from VpnService cannot
 * ride exec inheritance — it has to be passed as ancillary data
 * (SCM_RIGHTS) over a local socket after the spawn.
 *
 * The sidecar is listening (on tunFDSocketAddr in core/cmd/parvazd/fdrecv.go)
 * by the time it would normally print READY. Connect retries cover the
 * tiny window between fork+exec and the sidecar's listen() call.
 */
internal object TunFdSender {
    private const val TAG = "TunFdSender"

    /** Mirrors `tunFDSocketAddr` in core/cmd/parvazd/fdrecv.go. */
    private const val SOCKET_NAME = "parvaz/tun-fd"

    /**
     * Connect to the sidecar's abstract socket and send [fd] via
     * SCM_RIGHTS. Throws on failure after exhausting retries.
     */
    suspend fun send(fd: FileDescriptor) {
        val addr = LocalSocketAddress(SOCKET_NAME, LocalSocketAddress.Namespace.ABSTRACT)
        var lastErr: Exception? = null
        repeat(20) { attempt ->
            try {
                LocalSocket(LocalSocket.SOCKET_STREAM).use { sock ->
                    sock.connect(addr)
                    sock.setFileDescriptorsForSend(arrayOf(fd))
                    // Recvmsg on the sidecar requires at least one
                    // payload byte for the ancillary data to fly.
                    sock.outputStream.write(0)
                    sock.outputStream.flush()
                }
                Log.i(TAG, "sent TUN fd via SCM_RIGHTS (attempt=${attempt + 1})")
                return
            } catch (e: Exception) {
                lastErr = e
                delay(50)
            }
        }
        throw IllegalStateException("send TUN fd failed after retries: ${lastErr?.message}", lastErr)
    }
}
