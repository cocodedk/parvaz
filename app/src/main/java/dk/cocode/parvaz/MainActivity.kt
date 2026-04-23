package dk.cocode.parvaz

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import dk.cocode.parvaz.ui.theme.Ink
import dk.cocode.parvaz.ui.theme.InkSoft
import dk.cocode.parvaz.ui.theme.Paper
import dk.cocode.parvaz.ui.theme.ParvazTheme

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            ParvazTheme {
                Scaffold(
                    modifier = Modifier.fillMaxSize().background(Paper)
                ) { padding ->
                    PlaceholderScreen(modifier = Modifier.padding(padding))
                }
            }
        }
    }
}

@Composable
private fun PlaceholderScreen(modifier: Modifier = Modifier) {
    Column(
        modifier = modifier
            .fillMaxSize()
            .background(Paper)
            .padding(32.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(
            text = "§0 · SKELETON",
            style = MaterialTheme.typography.labelLarge,
            color = InkSoft,
        )
        Text(
            text = "Parvaz",
            style = MaterialTheme.typography.displayLarge,
            color = Ink,
        )
        Text(
            text = "CLEAR FOR FLIGHT — scaffolding only",
            style = MaterialTheme.typography.labelSmall,
            color = InkSoft,
        )
    }
}

@Preview(showBackground = true, backgroundColor = 0xFFF1E8D4)
@Composable
private fun PlaceholderPreview() {
    ParvazTheme { PlaceholderScreen() }
}
