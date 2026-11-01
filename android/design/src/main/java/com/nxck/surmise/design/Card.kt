package com.nxck.surmise.design

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawWithContent
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

/**
 * A raised surface. On a dark theme the card sits a step above the canvas
 * and catches light along its top edge, a one-pixel hairline in the
 * highlight token. On a light theme a white card on a grey canvas needs no
 * edge, so it gets a soft shadow instead.
 */
@Composable
fun Card(
    modifier: Modifier = Modifier,
    radius: Dp = Radii.card,
    padding: PaddingValues = PaddingValues(18.dp),
    content: @Composable BoxScope.() -> Unit,
) {
    val tokens = SurmiseTheme.tokens
    val shape = SquircleShape(radius)
    val lifted = if (tokens.dark) {
        modifier
    } else {
        modifier.shadow(
            elevation = 10.dp,
            shape = shape,
            ambientColor = tokens.shadow.copy(alpha = 0.04f),
            spotColor = tokens.shadow.copy(alpha = 0.08f),
        )
    }
    Box(
        modifier = lifted
            .clip(shape)
            .background(tokens.card)
            .drawWithContent {
                drawContent()
                if (tokens.dark) {
                    // Inset by the corner's reach, so the hairline fades into
                    // the curve rather than poking out of it.
                    val inset = radius.toPx()
                    drawLine(
                        color = tokens.highlight,
                        start = Offset(inset, 0.5f),
                        end = Offset(size.width - inset, 0.5f),
                        strokeWidth = 1.dp.toPx(),
                    )
                }
            }
            .padding(padding),
        content = content,
    )
}
