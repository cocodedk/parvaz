package dk.cocode.parvaz.ui.onboarding

import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts.StartActivityForResult
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import dk.cocode.parvaz.mitm.CaInstaller
import dk.cocode.parvaz.ui.theme.Paper
import dk.cocode.parvaz.vpn.CaGenerator
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

enum class CaInstallPhase {
    GENERATING, READY, AWAITING_INSTALL, VERIFYING, INSTALLED, FAILED, NO_SCREEN_LOCK,
}

/**
 * Step 3 of onboarding. Drives the MITM CA into Android's user-CA store:
 *  1. Pre-check KeyguardManager — without a screen lock Android refuses
 *     to install user CAs.
 *  2. Run `parvazd -gen-ca` to materialise the PEM under filesDir/parvaz-data.
 *  3. Launch ACTION_MANAGE_CA_CERTIFICATES with the DER payload.
 *  4. After the user returns from system UI, walk AndroidCAStore and
 *     verify by SHA-256 fingerprint — the Activity result code is unreliable.
 *
 * `phase` + `notified` survive configuration change and process death
 * via `rememberSaveable`; `caPem` is reloaded from disk on demand rather
 * than stashed in the bundle (the file is authoritative, parvazd -gen-ca
 * is idempotent).
 */
@Composable
fun CaInstallScreen(
    onInstalled: () -> Unit,
    modifier: Modifier = Modifier,
    generator: CaGenerator? = null,
    installer: CaInstaller? = null,
) {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    val controller = remember(generator, installer) {
        CaInstallController(
            context = context,
            generator = generator ?: CaGenerator(context),
            installer = installer ?: CaInstaller(context),
        )
    }

    var phase by rememberSaveable { mutableStateOf(CaInstallPhase.GENERATING) }
    var notified by rememberSaveable { mutableStateOf(false) }
    var caPem by remember { mutableStateOf<ByteArray?>(controller.loadPersistedCA()) }

    val launcher = rememberLauncherForActivityResult(StartActivityForResult()) {
        val pem = caPem ?: controller.loadPersistedCA()
        if (pem == null) { phase = CaInstallPhase.FAILED; return@rememberLauncherForActivityResult }
        caPem = pem
        phase = CaInstallPhase.VERIFYING
        scope.launch {
            phase = if (controller.isInstalled(pem)) CaInstallPhase.INSTALLED else CaInstallPhase.FAILED
        }
    }

    LaunchedEffect(Unit) {
        if (!controller.isDeviceSecure()) { phase = CaInstallPhase.NO_SCREEN_LOCK; return@LaunchedEffect }
        when (phase) {
            CaInstallPhase.VERIFYING -> {
                // Rotation killed our verification coroutine; isInstalled
                // is idempotent, just run it again.
                val pem = caPem ?: controller.loadPersistedCA()
                if (pem == null) { phase = CaInstallPhase.FAILED; return@LaunchedEffect }
                caPem = pem
                phase = if (controller.isInstalled(pem)) CaInstallPhase.INSTALLED else CaInstallPhase.FAILED
            }
            CaInstallPhase.AWAITING_INSTALL,
            CaInstallPhase.READY,
            CaInstallPhase.INSTALLED,
            CaInstallPhase.FAILED -> {
                // State preserved across recreation; ensure caPem is populated.
                caPem = caPem ?: controller.loadPersistedCA()
            }
            CaInstallPhase.NO_SCREEN_LOCK -> Unit
            CaInstallPhase.GENERATING -> {
                controller.materialiseCA().fold(
                    onSuccess = { pem -> caPem = pem; phase = CaInstallPhase.READY },
                    onFailure = { phase = CaInstallPhase.FAILED },
                )
            }
        }
    }

    LaunchedEffect(phase, notified) {
        if (phase == CaInstallPhase.INSTALLED && !notified) {
            notified = true
            delay(600)
            onInstalled()
        }
    }

    Column(
        modifier = modifier
            .fillMaxSize()
            .background(Paper)
            .padding(horizontal = 32.dp, vertical = 48.dp),
        verticalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        CaInstallHeader(phase)
        Spacer(Modifier.height(8.dp))
        CaInstallPrimary(phase) {
            when (phase) {
                CaInstallPhase.READY -> {
                    val pem = caPem ?: controller.loadPersistedCA()
                    if (pem == null) { phase = CaInstallPhase.FAILED; return@CaInstallPrimary }
                    controller.buildInstallIntent(pem).fold(
                        onSuccess = { intent ->
                            phase = CaInstallPhase.AWAITING_INSTALL
                            launcher.launch(intent)
                        },
                        onFailure = { phase = CaInstallPhase.FAILED },
                    )
                }
                CaInstallPhase.FAILED -> {
                    phase = CaInstallPhase.GENERATING
                    scope.launch {
                        controller.materialiseCA().fold(
                            onSuccess = { pem -> caPem = pem; phase = CaInstallPhase.READY },
                            onFailure = { phase = CaInstallPhase.FAILED },
                        )
                    }
                }
                else -> Unit
            }
        }
    }
}

