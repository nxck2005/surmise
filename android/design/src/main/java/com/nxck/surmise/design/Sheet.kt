package com.nxck.surmise.design

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.animate
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.Orientation
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.gestures.draggable
import androidx.compose.foundation.gestures.rememberDraggableState
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.onSizeChanged
import androidx.compose.ui.unit.dp

private const val SHEET_MS = 340

// A drag past this share of the sheet's height, or a flick faster than
// DISMISS_VELOCITY px/s, dismisses it; anything less springs back.
private const val DISMISS_SHARE = 0.25f
private const val DISMISS_VELOCITY = 1200f

/**
 * A bottom sheet over a dimmed screen. It slides up when [visible] turns
 * true and down when it turns false; a tap on the scrim or a drag down asks
 * the caller to hide it through [onDismiss]. The caller owns [visible], and
 * the back gesture, so a sheet is never dismissed behind its owner's back.
 *
 * Draw it last in a full-screen box, above what it covers.
 */
@Composable
fun Sheet(
    visible: Boolean,
    onDismiss: () -> Unit,
    modifier: Modifier = Modifier,
    content: @Composable ColumnScope.() -> Unit,
) {
    val tokens = SurmiseTheme.tokens
    val moving = SurmiseTheme.motion.enabled
    // 1 is fully up, 0 fully away.
    val shown = remember { Animatable(if (visible) 1f else 0f) }
    var drag by remember { mutableFloatStateOf(0f) }
    var height by remember { mutableIntStateOf(0) }

    LaunchedEffect(visible) {
        if (visible) drag = 0f
        val target = if (visible) 1f else 0f
        if (moving) shown.animateTo(target, tween(SHEET_MS, easing = SettleEasing)) else shown.snapTo(target)
    }
    if (!visible && shown.value == 0f) return

    Box(modifier.fillMaxSize()) {
        Box(
            Modifier
                .fillMaxSize()
                .graphicsLayer { alpha = shown.value }
                .background(tokens.scrim)
                .pointerInput(onDismiss) { detectTapGestures { onDismiss() } },
        )
        Column(
            modifier = Modifier
                .align(Alignment.BottomCenter)
                .fillMaxWidth()
                .onSizeChanged { height = it.height }
                .graphicsLayer { translationY = (1f - shown.value) * height + drag }
                .draggable(
                    orientation = Orientation.Vertical,
                    state = rememberDraggableState { d -> drag = (drag + d).coerceAtLeast(0f) },
                    onDragStopped = { velocity ->
                        if (drag > height * DISMISS_SHARE || velocity > DISMISS_VELOCITY) {
                            onDismiss()
                        } else {
                            animate(drag, 0f, animationSpec = tween(200, easing = SettleEasing)) { v, _ -> drag = v }
                        }
                    },
                )
                // Swallow taps on the sheet itself, so they never reach the scrim.
                .pointerInput(Unit) { detectTapGestures { } }
                .background(tokens.sheet, SquircleShape(Radii.sheet, bottom = false))
                .padding(start = 20.dp, end = 20.dp, top = 10.dp, bottom = 34.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Box(
                Modifier
                    .size(36.dp, 5.dp)
                    .background(tokens.ink.copy(alpha = 0.18f), CapsuleShape),
            )
            Spacer(Modifier.height(14.dp))
            Column(Modifier.fillMaxWidth(), content = content)
        }
    }
}
