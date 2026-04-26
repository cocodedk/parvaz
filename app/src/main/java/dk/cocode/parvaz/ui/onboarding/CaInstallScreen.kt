package dk.cocode.parvaz.ui.onboarding

import android.util.Log
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts.StartActivityForResult
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
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
import androidx.compose.ui.unit.dp
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import androidx.lifecycle.compose.LocalLifecycleOwner
import dk.cocode.parvaz.mitm.CaExporter
import dk.cocode.parvaz.mitm.CaInstaller
import dk.cocode.parvaz.mitm.SettingsLauncher
import dk.cocode.parvaz.ui.theme.Paper
import dk.cocode.parvaz.vpn.CaGenerator
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

private const val TAG = "CaInstall"
private const val VERIFY_GRACE_MS = 500L
private const val INSTALLED_CELEBRATE_MS = 600L

enum class CaInstallPhase {
    GENERATING, READY, AWAITING_INSTALL, VERIFYING, INSTALLED, FAILED, NO_SCREEN_LOCK,
}

/**
 * Manual CA install: exports the `.crt` to Downloads and walks the user
 * through Settings → Install from device storage. Fast-path: a
 * still-trusted on-disk CA short-circuits to INSTALLED on entry.
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
    val flavor = remember { SettingsLauncher.detectFlavor() }

    var phase by rememberSaveable { mutableStateOf(CaInstallPhase.GENERATING) }
    // In-memory only: a persisted latch would block onInstalled() if rotation hits during the celebrate delay.
    var notified by remember { mutableStateOf(false) }
    var caPem by remember { mutableStateOf<ByteArray?>(null) }
    var exportedCa by remember { mutableStateOf<CaExporter.ExportedCa?>(null) }

    fun verify(pem: ByteArray) = scope.launch {
        if (controller.isInstalled(pem)) { phase = CaInstallPhase.INSTALLED; return@launch }
        // Single retry after a grace window — `AndroidCAStore` enumeration
        // can lag the system install dialog by a few hundred ms.
        delay(VERIFY_GRACE_MS)
        phase = if (controller.isInstalled(pem)) {
            CaInstallPhase.INSTALLED
        } else {
            Log.w(TAG, "fingerprint not found in AndroidCAStore — flipping to FAILED")
            CaInstallPhase.FAILED
        }
    }

    // Prefer on-disk PEM over `parvazd -gen-ca` — the Go side's
    // LoadOrCreate is idempotent, so re-running on a returning user just
    // spawns a subprocess to re-read the same files.
    fun prepare(seed: ByteArray? = null) = scope.launch {
        phase = CaInstallPhase.GENERATING
        val pem = seed ?: controller.loadPersistedCA()
            ?: controller.materialiseCA().getOrElse { phase = CaInstallPhase.FAILED; return@launch }
        caPem = pem
        if (controller.isInstalled(pem)) { phase = CaInstallPhase.INSTALLED; return@launch }
        controller.export(pem).fold(
            onSuccess = { exp -> exportedCa = exp; phase = CaInstallPhase.READY },
            onFailure = { e ->
                Log.w(TAG, "CA export failed: ${e.message}")
                phase = CaInstallPhase.FAILED
            },
        )
    }

    val launcher = rememberLauncherForActivityResult(StartActivityForResult()) {
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
                val pem = caPem ?: controller.loadPersistedCA()
                if (pem == null) { phase = CaInstallPhase.FAILED; return@LaunchedEffect }
                caPem = pem
                verify(pem)
            }
            CaInstallPhase.AWAITING_INSTALL,
            CaInstallPhase.READY,
            CaInstallPhase.INSTALLED,
            CaInstallPhase.FAILED -> caPem = caPem ?: controller.loadPersistedCA()
            CaInstallPhase.NO_SCREEN_LOCK -> Unit
            CaInstallPhase.GENERATING -> prepare(seed = caPem ?: controller.loadPersistedCA())
        }
    }

    LaunchedEffect(phase) {
        if (phase == CaInstallPhase.INSTALLED && !notified) {
            notified = true
            delay(INSTALLED_CELEBRATE_MS)
            onInstalled()
        }
    }

    val lifecycleOwner = LocalLifecycleOwner.current
    DisposableEffect(lifecycleOwner) {
        val observer = LifecycleEventObserver { _, event ->
            if (event != Lifecycle.Event.ON_RESUME) return@LifecycleEventObserver
            when (phase) {
                CaInstallPhase.NO_SCREEN_LOCK -> if (controller.isDeviceSecure()) prepare()
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

    val onPrimary: () -> Unit = {
        when (phase) {
            CaInstallPhase.READY -> {
                phase = CaInstallPhase.AWAITING_INSTALL
                launcher.launch(SettingsLauncher.buildSecurityIntent(context.packageManager))
            }
            CaInstallPhase.FAILED -> { prepare(); Unit }
            else -> Unit
        }
    }
    val onShowFile: (() -> Unit)? = exportedCa?.let { exp ->
        { launcher.launch(SettingsLauncher.buildViewCertFileIntent(exp.contentUri)) }
    }
    val showSteps = phase in setOf(
        CaInstallPhase.READY,
        CaInstallPhase.AWAITING_INSTALL,
        CaInstallPhase.FAILED,
    )
    val showShowFile = phase in setOf(CaInstallPhase.READY, CaInstallPhase.AWAITING_INSTALL)

    Column(
        modifier = modifier
            .fillMaxSize()
            .background(Paper)
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 32.dp, vertical = 48.dp),
        verticalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        CaInstallHeader(phase)
        Spacer(Modifier.height(8.dp))
        if (showSteps) {
            CaInstallSteps(flavor = flavor, autoAdvance = phase != CaInstallPhase.FAILED)
            Spacer(Modifier.height(4.dp))
        }
        CaInstallPrimary(phase, onClick = onPrimary)
        if (showShowFile) CaInstallShowFile(onShowFile)
    }
}
