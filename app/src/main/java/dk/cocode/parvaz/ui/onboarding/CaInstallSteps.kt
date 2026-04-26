package dk.cocode.parvaz.ui.onboarding

import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.ui.draw.drawBehind
import kotlinx.coroutines.delay
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalLayoutDirection
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.LayoutDirection
import androidx.compose.ui.unit.dp
import dk.cocode.parvaz.R
import dk.cocode.parvaz.mitm.SettingsFlavor
import dk.cocode.parvaz.ui.theme.Burnt
import dk.cocode.parvaz.ui.theme.Ink
import dk.cocode.parvaz.ui.theme.InkSoft
import dk.cocode.parvaz.ui.theme.Paper
import dk.cocode.parvaz.ui.theme.Rule

private const val PULSE_PERIOD_MS = 1200
private const val STEP_TICK_MS = 3_000L

/**
 * Five numbered cards covering the manual install path. The active
 * step pulses Burnt-on-Paper to draw the eye; when [autoAdvance] is
 * true the active index ticks every [STEP_TICK_MS] so the screen feels
 * alive while the user navigates Settings. Persian-Indic numerals are
 * baked into the string resources; LayoutDirection is forced to RTL.
 */
@Composable
fun CaInstallSteps(
    flavor: SettingsFlavor,
    autoAdvance: Boolean,
    modifier: Modifier = Modifier,
) {
    var activeStep by remember { mutableIntStateOf(0) }
    LaunchedEffect(autoAdvance) {
        if (!autoAdvance) {
            // FAILED + similar paused states reset the focus to step 1 so
            // the user retries from the top rather than mid-sequence.
            activeStep = 0
            return@LaunchedEffect
        }
        while (true) {
            delay(STEP_TICK_MS)
            activeStep = (activeStep + 1) % flavor.stepLabels.size
        }
    }
    val pulse = rememberInfiniteTransition(label = "ca-step-pulse")
    val glow by pulse.animateFloat(
        initialValue = 0f,
        targetValue = 1f,
        animationSpec = infiniteRepeatable(
            animation = tween(durationMillis = PULSE_PERIOD_MS, easing = LinearEasing),
            repeatMode = RepeatMode.Reverse,
        ),
        label = "ca-step-pulse-alpha",
    )

    // Force RTL so step bullets sit on the right edge — matches the
    // Farsi reading direction even when the host app forces LTR for
    // testing.
    CompositionLocalProvider(LocalLayoutDirection provides LayoutDirection.Rtl) {
        Column(
            modifier = modifier
                .fillMaxWidth()
                .testTag(TestTags.CaInstallSteps),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            for (index in flavor.stepLabels.indices) {
                StepCard(
                    iconRes = STEP_ICONS[index],
                    label = stringResource(flavor.stepLabels[index]),
                    isActive = index == activeStep,
                    glow = glow,
                )
            }
            Spacer(Modifier.size(4.dp))
            Text(
                text = stringResource(R.string.ca_install_steps_fallback),
                style = MaterialTheme.typography.bodySmall,
                color = InkSoft,
                modifier = Modifier.padding(horizontal = 8.dp),
            )
        }
    }
}

@Composable
private fun StepCard(
    iconRes: Int,
    label: String,
    isActive: Boolean,
    glow: Float,
) {
    val borderColor by animateColorAsState(
        targetValue = if (isActive) Burnt else Rule,
        label = "ca-step-border",
    )
    // Active-step background: Burnt at ~12% alpha, oscillating to ~3%.
    // Painted via drawBehind so the alpha read happens at draw phase, not
    // recomposition. Inactive: flat Paper.
    val tintAlpha = if (isActive) 0.03f + 0.09f * glow else 0f
    val backgroundModifier = if (isActive) {
        Modifier.drawBehind { drawRect(Burnt.copy(alpha = tintAlpha)) }
    } else {
        Modifier.background(Paper)
    }
    Row(
        verticalAlignment = Alignment.CenterVertically,
        modifier = Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(4.dp))
            .then(backgroundModifier)
            .border(width = 1.dp, color = borderColor, shape = RoundedCornerShape(4.dp))
            .padding(horizontal = 12.dp, vertical = 10.dp),
    ) {
        Icon(
            painter = painterResource(id = iconRes),
            contentDescription = null,
            tint = Color.Unspecified,
            modifier = Modifier.size(28.dp),
        )
        Spacer(Modifier.width(12.dp))
        Text(
            text = label,
            style = MaterialTheme.typography.bodyLarge,
            color = if (isActive) Ink else InkSoft,
            fontWeight = if (isActive) FontWeight.SemiBold else FontWeight.Normal,
            modifier = Modifier.fillMaxWidth(),
        )
    }
}

private val STEP_ICONS = intArrayOf(
    R.drawable.ic_step_lock,
    R.drawable.ic_step_folder,
    R.drawable.ic_step_cert,
    R.drawable.ic_step_pick,
    R.drawable.ic_step_back,
)
