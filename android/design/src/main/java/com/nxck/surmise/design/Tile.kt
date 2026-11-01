package com.nxck.surmise.design

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.FastOutLinearInEasing
import androidx.compose.animation.core.LinearOutSlowInEasing
import androidx.compose.animation.core.keyframes
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.drawscope.translate
import androidx.compose.ui.graphics.drawOutline
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.lerp
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

/** What a board tile shows. */
enum class TileState {
    /** A row not reached yet. */
    Empty,

    /** An empty tile in the row being typed. */
    Ready,

    /** A typed letter, not yet scored. */
    Typed,

    /** Where the next letter goes. */
    Cursor,

    Correct,
    Present,
    Absent;

    val scored: Boolean get() = this == Correct || this == Present || this == Absent
}

/** How long one tile's flip takes, and the gap between neighbours. */
const val FLIP_MS = 520
const val FLIP_STAGGER_MS = 100
private const val HOP_MS = 320

/** A tile's own width and height, and the gap between tiles on a board. */
val TileSize = 60.dp
val TileGap = 6.dp

/**
 * One board tile. When [state] turns from unscored to scored the tile flips
 * on its horizontal axis and shows its mark at the half-way point, after
 * [flipDelayMs] (a row passes `index * FLIP_STAGGER_MS`); [onFlipped] runs
 * when it lands. A newly typed letter hops. With motion off the mark shows at
 * once and [onFlipped] still runs, so a caller waiting on it never stalls.
 */
@Composable
fun Tile(
    letter: Char?,
    state: TileState,
    modifier: Modifier = Modifier,
    size: Dp = TileSize,
    flipDelayMs: Int = 0,
    onFlipped: () -> Unit = {},
) {
    val tokens = SurmiseTheme.tokens
    val moving = SurmiseTheme.motion.enabled
    val done by rememberUpdatedState(onFlipped)

    // The face on show lags [state] while a flip is under way.
    var face by remember { mutableStateOf(state) }
    val turn = remember { Animatable(0f) }
    val hop = remember { Animatable(1f) }

    LaunchedEffect(state) {
        val flips = state.scored && !face.scored
        when {
            flips && moving -> {
                // The delay rides on the animation, so it runs on the frame
                // clock like the flip itself.
                turn.animateTo(90f, tween(FLIP_MS / 2, delayMillis = flipDelayMs, easing = FastOutLinearInEasing))
                face = state
                turn.snapTo(-90f)
                turn.animateTo(0f, tween(FLIP_MS / 2, easing = LinearOutSlowInEasing))
                done()
            }
            flips -> {
                face = state
                done()
            }
            else -> {
                val typed = state == TileState.Typed && face != TileState.Typed
                face = state
                if (typed && moving) {
                    hop.animateTo(1f, keyframes {
                        durationMillis = HOP_MS
                        1.08f at HOP_MS * 4 / 10
                        1f at HOP_MS
                    })
                }
            }
        }
    }

    val shape = SquircleShape(Radii.tile * (size / TileSize))
    val look = tileLook(face, tokens)
    Box(
        modifier = modifier
            .size(size)
            .graphicsLayer {
                rotationX = turn.value
                cameraDistance = 12f * density
                scaleX = hop.value
                scaleY = hop.value
            }
            .drawBehind {
                if (look.halo != Color.Transparent) {
                    // A soft ring just outside the border.
                    val grow = 3.dp.toPx()
                    val ring = SquircleShape(Radii.tile + 3.dp)
                        .createOutline(Size(this.size.width + grow * 2, this.size.height + grow * 2), layoutDirection, this)
                    translate(-grow, -grow) { drawOutline(ring, look.halo) }
                }
            }
            .background(look.fill, shape)
            .then(if (look.border != Color.Transparent) Modifier.border(2.dp, look.border, shape) else Modifier),
        contentAlignment = Alignment.Center,
    ) {
        if (letter != null) {
            SurmiseText(
                text = letter.uppercaseChar().toString(),
                style = SurmiseTheme.type.tile.copy(fontSize = SurmiseTheme.type.tile.fontSize * (size / TileSize)),
                color = look.letter,
            )
        }
    }
}

private data class TileLook(val fill: Color, val border: Color, val letter: Color, val halo: Color = Color.Transparent)

private fun tileLook(state: TileState, t: SurmiseTokens): TileLook = when (state) {
    TileState.Empty -> TileLook(Color.Transparent, t.tileEmpty, t.ink)
    TileState.Ready -> TileLook(t.ink.copy(alpha = 0.03f), t.ink.copy(alpha = 0.20f), t.ink)
    // Typed tiles sit a step up from the canvas, most of the way to a card.
    TileState.Typed -> TileLook(lerp(t.canvas, t.card, 0.65f), t.tileTyped, t.ink)
    TileState.Cursor -> TileLook(t.cursor.copy(alpha = 0.06f), t.cursor, t.ink, halo = t.cursor.copy(alpha = 0.16f))
    TileState.Correct -> TileLook(t.tiles.correct.fill, Color.Transparent, t.tiles.correct.letter)
    TileState.Present -> TileLook(t.tiles.present.fill, Color.Transparent, t.tiles.present.letter)
    TileState.Absent -> TileLook(t.tiles.absent.fill, Color.Transparent, t.tiles.absent.letter)
}
