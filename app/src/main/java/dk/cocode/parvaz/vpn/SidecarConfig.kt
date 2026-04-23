package dk.cocode.parvaz.vpn

import dk.cocode.parvaz.settings.Access

/**
 * SidecarConfig is what the Kotlin app sends to the Go sidecar's stdin
 * (one JSON object, one line). The Go side parses it with
 * `encoding/json` into `parvazd.Config` — see `core/cmd/parvazd/main.go`.
 *
 * JSON field names match parvazd's struct tags exactly. Break at your
 * peril.
 */
data class SidecarConfig(
    val access: Access,
    val dataDir: String,
    val googleIP: String = "216.239.38.120",
    val frontDomain: String = "www.google.com",
    val listenHost: String = "127.0.0.1",
    val listenPort: Int = 1080,
    /**
     * Raw TUN file descriptor inherited from the JVM. Zero means "no
     * TUN, SOCKS5-only path" (used by the e2e harness). Positive means
     * the sidecar runs tun2socks on this fd; Kotlin MUST have cleared
     * FD_CLOEXEC and detached the ParcelFileDescriptor first.
     */
    val tunFD: Int = 0,
    /** MTU of the TUN interface; must match VpnService.Builder.setMtu. */
    val tunMTU: Int = 0,
) {
    /** Serialize to a single-line JSON string suitable for stdin. */
    fun toJson(): String = buildString {
        append('{')
        kvStringArray("script_urls", listOf(access.deploymentURL)); comma()
        kvString("auth_key", access.accessKey); comma()
        kvString("google_ip", googleIP); comma()
        kvString("front_domain", frontDomain); comma()
        kvString("listen_host", listenHost); comma()
        kvInt("listen_port", listenPort); comma()
        kvInt("tun_fd", tunFD); comma()
        kvInt("tun_mtu", tunMTU); comma()
        kvString("data_dir", dataDir)
        append('}')
    }

    private fun StringBuilder.kvString(k: String, v: String) {
        append('"').append(k).append("\":\"").append(jsonEscape(v)).append('"')
    }

    private fun StringBuilder.kvInt(k: String, v: Int) {
        append('"').append(k).append("\":").append(v)
    }

    private fun StringBuilder.kvStringArray(k: String, v: List<String>) {
        append('"').append(k).append("\":[")
        v.forEachIndexed { i, s ->
            if (i > 0) append(',')
            append('"').append(jsonEscape(s)).append('"')
        }
        append(']')
    }

    private fun StringBuilder.comma() {
        append(',')
    }

    private fun jsonEscape(s: String): String {
        val b = StringBuilder(s.length + 8)
        for (c in s) {
            when (c) {
                '\\' -> b.append("\\\\")
                '"' -> b.append("\\\"")
                '\n' -> b.append("\\n")
                '\r' -> b.append("\\r")
                '\t' -> b.append("\\t")
                else -> if (c.code < 0x20) {
                    b.append("\\u").append("%04x".format(c.code))
                } else {
                    b.append(c)
                }
            }
        }
        return b.toString()
    }
}
