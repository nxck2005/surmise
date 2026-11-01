package com.nxck.surmise.design

import androidx.compose.foundation.Image
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ColorFilter
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.graphics.vector.addPathNodes
import androidx.compose.ui.graphics.vector.rememberVectorPainter
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

// The app's icons: thin round-capped strokes, the mockups' paths verbatim.
// Each is drawn in black and tinted where it is used, so this file is the
// other one allowed to name a colour; the black never reaches the screen.

private fun stroke(name: String, d: String, w: Float = 24f, h: Float = 24f, width: Float = 1.75f): ImageVector =
    ImageVector.Builder(name = name, defaultWidth = w.dp, defaultHeight = h.dp, viewportWidth = w, viewportHeight = h)
        .addPath(
            pathData = addPathNodes(d),
            fill = null,
            stroke = SolidColor(Color.Black),
            strokeLineWidth = width,
            strokeLineCap = StrokeCap.Round,
            strokeLineJoin = StrokeJoin.Round,
        )
        .build()

object SurmiseIcons {
    // Tab bar.
    val Play = stroke("play", "M4 4h6v6H4zM14 4h6v6h-6zM4 14h6v6H4zM14 14h6v6h-6z", width = 1.9f)
    val History = stroke("history", "M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01", width = 1.9f)
    val Record = stroke("record", "M4 20V10M10 20V4M16 20v-7M22 20H2", width = 1.9f)
    val Settings = stroke("settings", "M4 7h10M18 7h2M4 17h4M12 17h8M16 5v4M10 15v4", width = 1.9f)

    // Navigation.
    val Back = stroke("back", "M7 1L1 7l6 6", w = 8f, h = 14f)
    val Chevron = stroke("chevron", "M1 1l5 5-5 5", w = 7f, h = 12f)
    val ArrowRight = stroke("arrow right", "M5 12h14M13 6l6 6-6 6")

    // The keyboard.
    val Backspace = stroke("backspace", "M7 1h13a1 1 0 0 1 1 1v12a1 1 0 0 1-1 1H7L1 8zM11 5l6 6M17 5l-6 6", w = 22f, h = 16f, width = 1.6f)

    // Modes.
    val Custom = stroke("custom", "M12 20h9M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4z")
    val Sprint = stroke("sprint", "M12 21a8 8 0 1 0 0-16 8 8 0 0 0 0 16M12 9v4l2 2M10 2h4")
    val Rooms = stroke("rooms", "M16 19v-1a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v1M9 10a3 3 0 1 0 0-6 3 3 0 0 0 0 6M22 19v-1a4 4 0 0 0-3-3.9M16 4.1a3 3 0 0 1 0 5.8")

    // Actions and settings rows.
    val Check = stroke("check", "M5 12l5 5L20 7", width = 3.5f)
    val Share = stroke("share", "M4 12v7a1 1 0 0 0 1 1h14a1 1 0 0 0 1-1v-7M16 6l-4-4-4 4M12 2v13")
    val Download = stroke("download", "M12 3v12M7 10l5 5 5-5M4 21h16")
    val Restart = stroke("restart", "M3 12a9 9 0 1 0 3-6.7L3 8M3 3v5h5")
    val Theme = stroke("theme", "M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9z")
    val Info = stroke("info", "M12 22a10 10 0 1 0 0-20 10 10 0 0 0 0 20M12 16v-4M12 8h.01")
    val Help = stroke("help", "M12 22a10 10 0 1 0 0-20 10 10 0 0 0 0 20M9.1 9a3 3 0 0 1 5.8 1c0 2-3 3-3 3M12 17h.01")
    val Globe = stroke("globe", "M2 12h20M12 2a15 15 0 0 1 0 20M12 2a15 15 0 0 0 0 20M12 22a10 10 0 1 0 0-20 10 10 0 0 0 0 20")
}

/** An icon tinted with a token. [size] is the box; the icon keeps its aspect. */
@Composable
fun Icon(icon: ImageVector, color: Color, modifier: Modifier = Modifier, size: Dp = 20.dp, description: String? = null) {
    Image(
        painter = rememberVectorPainter(icon),
        contentDescription = description,
        modifier = modifier.size(size),
        colorFilter = ColorFilter.tint(color),
    )
}
