package com.nxck.surmise.design

import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.CubicBezierEasing
import androidx.compose.animation.core.animateDpAsState
import androidx.compose.animation.core.snap
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.dp

/** The ease every small control settles on: a fast start, a long soft stop. */
val SettleEasing = CubicBezierEasing(0.22f, 1f, 0.36f, 1f)

private val TrackWidth = 50.dp
private val TrackHeight = 30.dp
private val Knob = 24.dp
private val KnobInset = 3.dp

/** An on/off switch: orange when on, a neutral track when off. */
@Composable
fun Switch(
    checked: Boolean,
    onCheckedChange: (Boolean) -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
) {
    val tokens = SurmiseTheme.tokens
    val moving = SurmiseTheme.motion.enabled
    val x by animateDpAsState(
        targetValue = if (checked) TrackWidth - Knob - KnobInset else KnobInset,
        animationSpec = if (moving) tween(200, easing = SettleEasing) else snap(),
        label = "knob",
    )
    val track by animateColorAsState(
        targetValue = if (checked) tokens.action else tokens.chartNeutral,
        animationSpec = if (moving) tween(200, easing = SettleEasing) else snap(),
        label = "track",
    )
    // The knob is the brightest thing on the row in either appearance.
    val knob = if (tokens.dark) tokens.ink else tokens.card
    Box(
        modifier = modifier
            .pressable(onClick = { onCheckedChange(!checked) }, enabled = enabled, role = Role.Switch)
            .size(TrackWidth, TrackHeight)
            .clip(CapsuleShape)
            .background(track),
        contentAlignment = Alignment.CenterStart,
    ) {
        Box(
            Modifier
                .offset { IntOffset(x.roundToPx(), 0) }
                .size(Knob)
                .shadow(2.dp, CapsuleShape, ambientColor = tokens.shadow, spotColor = tokens.shadow)
                .background(knob, CapsuleShape),
        )
    }
}
