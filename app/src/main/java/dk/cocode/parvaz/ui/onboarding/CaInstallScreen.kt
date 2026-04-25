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
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalLifecycleOwner
import androidx.compose.ui.unit.dp
import android.util.Log
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import dk.cocode.parvaz.mitm.CaInstaller
import dk.cocode.parvaz.ui.theme.Paper
import dk.cocode.parvaz.vpn.CaGenerator
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

private const val TAG = "CaInstall"

enum class CaInstallPhase {
    GENERATING, READY, AWAITING_INSTALL, VERIFYING, INSTALLED, UNVERIFIED, FAILED, NO_SCREEN_LOCK,
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
    // In-memory only: a persisted latch would block onInstalled() if rotation hits during the 600ms delay.
    var notified by remember { mutableStateOf(false) }
    var caPem by remember { mutableStateOf<ByteArray?>(controller.loadPersistedCA()) }

    fun verify(pem: ByteArray) = scope.launch {
        phase = if (controller.isInstalled(pem)) {
            CaInstallPhase.INSTALLED
        } else {
            // Samsung + some OEMs on API 34 hide user CAs from AndroidCAStore in non-system processes;
            // UNVERIFIED lets the user continue (browsers use a separate codepath).
            Log.w(TAG, "AndroidCAStore walk could not confirm install; entering UNVERIFIED")
            CaInstallPhase.UNVERIFIED
        }
    }

    fun generate() = scope.launch {
        phase = CaInstallPhase.GENERATING
        controller.materialiseCA().fold(
            onSuccess = { pem -> caPem = pem; phase = CaInstallPhase.READY },
            onFailure = { phase = CaInstallPhase.FAILED },
        )
    }

    val launcher = rememberLauncherForActivityResult(StartActivityForResult()) {
        // ON_RESUME may have already advanced phase (process-death path).
        // Only act if we're still the first to observe the result.
        if (phase != CaInstallPhase.AWAITING_INSTALL) return@rememberLauncherForActivityResult
        val pem = caPem ?: controller.loadPersistedCA()
        if (pem == null) { phase = CaInstallPhase.FAILED; return@rememberLauncherForActivityResult }
        caPem = pem
        phase = CaInstallPhase.VERIFYING
        verify(pem)
    }

    LaunchedEffect(Unit) {
        if (!controller.isDeviceSecure()) { phase = CaInstallPhase.NO_SCREEN_LOCK; return@LaunchedEffect }
        when (phase) {
            CaInstallPhase.VERIFYING -> {
                // Rotation killed prior verification coroutine; isInstalled is idempotent.
                val pem = caPem ?: controller.loadPersistedCA()
                if (pem == null) { phase = CaInstallPhase.FAILED; return@LaunchedEffect }
                caPem = pem
                verify(pem)
            }
            CaInstallPhase.AWAITING_INSTALL,
            CaInstallPhase.READY,
            CaInstallPhase.INSTALLED,
            CaInstallPhase.UNVERIFIED,
            CaInstallPhase.FAILED -> {
                caPem = caPem ?: controller.loadPersistedCA()
            }
            CaInstallPhase.NO_SCREEN_LOCK -> Unit
            CaInstallPhase.GENERATING -> generate()
        }
    }

    // Not keyed on `notified` — that would cancel the delay before onInstalled() fires.
    LaunchedEffect(phase) {
        if (phase == CaInstallPhase.INSTALLED && !notified) {
            notified = true
            delay(600)
            onInstalled()
        }
    }

    // Recover stuck states on ON_RESUME:
    //  • NO_SCREEN_LOCK → user enabled a lock in Settings, then returned.
    //  • AWAITING_INSTALL → process was killed mid-CA-install; the
    //    ActivityResultLauncher callback tied to the old Activity will
    //    never fire, so re-verify against AndroidCAStore ourselves.
    val lifecycleOwner = LocalLifecycleOwner.current
    DisposableEffect(lifecycleOwner) {
        val observer = LifecycleEventObserver { _, event ->
            if (event != Lifecycle.Event.ON_RESUME) return@LifecycleEventObserver
            when (phase) {
                CaInstallPhase.NO_SCREEN_LOCK -> if (controller.isDeviceSecure()) generate()
                CaInstallPhase.AWAITING_INSTALL -> {
                    val pem = caPem ?: controller.loadPersistedCA()
                    if (pem == null) phase = CaInstallPhase.FAILED
                    else { caPem = pem; phase = CaInstallPhase.VERIFYING; verify(pem) }
                }
                else -> Unit
            }
        }
        lifecycleOwner.lifecycle.addObserver(observer)
        onDispose { lifecycleOwner.lifecycle.removeObserver(observer) }
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
                // UNVERIFIED retries the system intent with the CA on disk — no re-generation.
                CaInstallPhase.READY, CaInstallPhase.UNVERIFIED -> {
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
                CaInstallPhase.FAILED -> generate()
                else -> Unit
            }
        }
        // User asserts cert is installed despite our inability to verify; promote to INSTALLED.
        CaInstallContinue(phase, onContinue = { phase = CaInstallPhase.INSTALLED })
    }
}

