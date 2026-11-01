package com.nxck.surmise

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.text.BasicText
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.tooling.preview.Preview
import com.nxck.surmise.design.SurmiseTheme

// Placeholder is the only screen until the home screen lands (P5). It shows
// that the app, the design module and the generated app name are wired up.
@Composable
fun Placeholder(modifier: Modifier = Modifier) {
    val tokens = SurmiseTheme.tokens
    Column(
        modifier = modifier
            .fillMaxSize()
            .background(tokens.canvas)
            .safeDrawingPadding(),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        BasicText(stringResource(R.string.app_name), style = SurmiseTheme.type.title)
        BasicText("coming soon", style = SurmiseTheme.type.caption)
    }
}

@Preview
@Composable
private fun PlaceholderPreview() {
    SurmiseTheme { Placeholder() }
}
