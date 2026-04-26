package dk.cocode.parvaz.onboarding

import android.content.Context
import android.net.VpnService
import dk.cocode.parvaz.mitm.CaFingerprint
import dk.cocode.parvaz.mitm.CaInstaller
import dk.cocode.parvaz.settings.Access
import dk.cocode.parvaz.settings.ParvazDataDir
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.File

/**
 * Re-validates that a stored "onboarding complete" flag still matches
 * the device state: the Parvaz CA must still be in AndroidCAStore and
 * VpnService permission must still be granted. If either has been
 * revoked (cert wiped, permission denied, factory reset, etc.) callers
 * should send the user back into onboarding instead of the connect
 * screen, which would otherwise fail the moment they tap پرواز.
 */
suspend fun isOnboardingStillReady(context: Context, access: Access?): Boolean {
    if (access == null) return false
    return runCatching {
        hasInstalledParvazCa(context) && VpnService.prepare(context) == null
    }.getOrDefault(false)
}

private suspend fun hasInstalledParvazCa(context: Context): Boolean {
    return runCatching {
        val caPem = withContext(Dispatchers.IO) {
            val f = File(ParvazDataDir.forContext(context), "ca/ca.crt")
            f.takeIf { it.isFile }?.readBytes()
        } ?: return false
        val caDer = CaFingerprint.pemToDer(caPem)
        CaInstaller(context).isInstalled(caDer)
    }.getOrDefault(false)
}
