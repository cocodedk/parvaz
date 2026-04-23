package dk.cocode.parvaz.ui.theme

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable

// Light-only, not dynamic — NOTAM parchment always wins.
private val ParvazColorScheme = lightColorScheme(
    primary = Oxblood,
    onPrimary = Paper,
    primaryContainer = PaperDeep,
    onPrimaryContainer = Ink,
    secondary = Burnt,
    onSecondary = Paper,
    tertiary = Olive,
    onTertiary = Paper,
    background = Paper,
    onBackground = Ink,
    surface = PaperAlt,
    onSurface = Ink,
    surfaceVariant = PaperDeep,
    onSurfaceVariant = InkSoft,
    outline = InkFaint,
    outlineVariant = Rule,
    error = Oxblood,
    onError = Paper,
)

@Composable
fun ParvazTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = ParvazColorScheme,
        typography = Typography,
        content = content
    )
}
