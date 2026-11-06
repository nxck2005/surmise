package com.nxck.surmise.design

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.defaultMinSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp

enum class CapsuleStyle {
    /** The one thing to do on a screen: orange, dark text. */
    Primary,

    /** Everything else: a quiet fill in the highlight token. */
    Secondary,
}

/**
 * A pill button. Disabled, it turns to the quiet fill with muted text and
 * stops taking presses: a faded orange still reads as something to press.
 */
@Composable
fun CapsuleButton(
    label: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    style: CapsuleStyle = CapsuleStyle.Primary,
    enabled: Boolean = true,
) {
    val tokens = SurmiseTheme.tokens
    val quiet = if (tokens.dark) tokens.highlight else tokens.chartNeutral
    val (fill, text) = when {
        !enabled -> quiet to tokens.inkMuted
        style == CapsuleStyle.Primary -> tokens.action to tokens.onAction
        else -> quiet to tokens.ink
    }
    Box(
        modifier = modifier
            .pressable(onClick = onClick, enabled = enabled)
            .height(40.dp)
            .defaultMinSize(minWidth = 64.dp)
            .clip(CapsuleShape)
            .background(fill)
            .padding(horizontal = 18.dp),
        contentAlignment = Alignment.Center,
    ) {
        SurmiseText(
            text = label,
            style = SurmiseTheme.type.body.copy(fontWeight = FontWeight.SemiBold),
            color = text,
            maxLines = 1,
        )
    }
}
