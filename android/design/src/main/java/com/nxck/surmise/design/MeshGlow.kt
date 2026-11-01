package com.nxck.surmise.design

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clipToBounds
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.scale
import androidx.compose.ui.graphics.lerp

/**
 * The daily hero's light: a blue glow from the top left and an orange one
 * from the bottom right, over a surface a shade off the canvas. The colours
 * are the theme's glowFrom and glowTo, so a theme can recolour the hero.
 */
@Composable
fun MeshGlow(modifier: Modifier = Modifier, content: @Composable BoxScope.() -> Unit = {}) {
    val tokens = SurmiseTheme.tokens
    // Dark: a deep blue-black between the canvas and a card. Light: the card.
    val base = if (tokens.dark) lerp(tokens.canvas, tokens.card, 0.7f) else tokens.card
    val from = tokens.glowFrom.copy(alpha = if (tokens.dark) 0.42f else 0.22f)
    val to = tokens.glowTo.copy(alpha = if (tokens.dark) 0.30f else 0.20f)
    Box(
        modifier = modifier.clipToBounds().drawBehind {
            drawRect(base)
            glow(from, Offset.Zero, 0.90f, 0.80f, 0.62f)
            glow(to, Offset(size.width, size.height), 0.85f, 0.75f, 0.60f)
        },
        content = content,
    )
}

// A CSS radial-gradient(<rw> <rh> at <centre>, colour, transparent <stop>):
// an ellipse of the box's width × rw and height × rh, drawn as a circle
// stretched on one axis.
private fun DrawScope.glow(color: Color, centre: Offset, rw: Float, rh: Float, stop: Float) {
    val rx = size.width * rw
    val ry = size.height * rh
    scale(scaleX = 1f, scaleY = ry / rx, pivot = centre) {
        drawCircle(
            brush = Brush.radialGradient(
                0f to color,
                stop to color.copy(alpha = 0f),
                center = centre,
                radius = rx,
            ),
            radius = rx,
            center = centre,
        )
    }
}
