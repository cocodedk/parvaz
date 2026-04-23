package dk.cocode.parvaz.vpn

import dk.cocode.parvaz.settings.Access
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class SidecarConfigTest {
    @Test
    fun jsonShape_matchesParvazdStructTags() {
        val cfg = SidecarConfig(
            access = Access("DEP123", "KEY456", "My Phone"),
            dataDir = "/data/data/dk.cocode.parvaz/files/sidecar",
        )
        val json = cfg.toJson()

        // Deterministic field order (makes golden-string asserts cheap and
        // mirrors the Go struct literal order in parvazd.Config).
        assertEquals(
            """{"script_urls":["https://script.google.com/macros/s/DEP123/exec"],""" +
                """"auth_key":"KEY456",""" +
                """"google_ip":"216.239.38.120",""" +
                """"front_domain":"www.google.com",""" +
                """"listen_host":"127.0.0.1",""" +
                """"listen_port":1080,""" +
                """"data_dir":"/data/data/dk.cocode.parvaz/files/sidecar"}""",
            json,
        )
    }

    @Test
    fun jsonEscape_handlesQuotesAndBackslashes() {
        val cfg = SidecarConfig(
            access = Access("DEP", "key\"with\\edge", null),
            dataDir = "/tmp",
        )
        val json = cfg.toJson()
        // Escaped quote + backslash survive the round-trip format.
        assertTrue("auth_key escaping broken: $json",
            json.contains(""""auth_key":"key\"with\\edge""""))
    }

    @Test
    fun overridesRespected() {
        val cfg = SidecarConfig(
            access = Access("D", "K", null),
            dataDir = "/x",
            googleIP = "64.233.160.0",
            frontDomain = "www.gstatic.com",
            listenPort = 1081,
        )
        val json = cfg.toJson()
        assertTrue(json.contains(""""google_ip":"64.233.160.0""""))
        assertTrue(json.contains(""""front_domain":"www.gstatic.com""""))
        assertTrue(json.contains(""""listen_port":1081"""))
    }
}
