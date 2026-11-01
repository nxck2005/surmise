package com.nxck.surmise.board

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.keyframes
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.focusable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.input.key.utf16CodePoint
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.min
import com.nxck.surmise.R
import com.nxck.surmise.TopBar
import com.nxck.surmise.boardTitle
import com.nxck.surmise.clockText
import com.nxck.surmise.core.gen.BoardView
import com.nxck.surmise.design.CapsuleButton
import com.nxck.surmise.design.CapsuleShape
import com.nxck.surmise.design.CapsuleStyle
import com.nxck.surmise.design.FLIP_STAGGER_MS
import com.nxck.surmise.design.Keyboard
import com.nxck.surmise.design.SurmiseText
import com.nxck.surmise.design.SurmiseTheme
import com.nxck.surmise.design.Tile
import com.nxck.surmise.design.TileGap
import com.nxck.surmise.design.TileSize
import com.nxck.surmise.design.TileState
import com.nxck.surmise.design.rememberHaptics
import com.nxck.surmise.keyStateOf
import com.nxck.surmise.refusalText
import com.nxck.surmise.tileStateOf
import kotlinx.coroutines.delay

/** How long a refusal stays on screen. */
private const val NOTE_MS = 1800L

/** The gap between neighbours in a win's hop, and one hop's length. */
private const val HOP_STAGGER_MS = 80
private const val HOP_MS = 300

/** The board screen: the grid, a line for notes, the keyboard and the result sheet. */
@Composable
fun BoardScreen(model: BoardModel, onBack: () -> Unit, onHome: () -> Unit, onShare: (String) -> Unit) {
    val v = model.view
    val context = LocalContext.current
    val haptics = rememberHaptics()
    val moving = SurmiseTheme.motion.enabled
    val tokens = SurmiseTheme.tokens

    // The keys show what the board showed before the row now turning, so a
    // key never gives a tile's mark away before the tile does.
    var keys by remember { mutableStateOf(v.keys) }
    if (model.revealing < 0 && keys != v.keys) keys = v.keys

    val shake = remember { Animatable(0f) }
    LaunchedEffect(model.shakes) {
        if (model.shakes == 0) return@LaunchedEffect
        haptics.reject()
        if (moving) {
            shake.animateTo(0f, keyframes {
                durationMillis = 360
                -8f at 72
                8f at 144
                -6f at 216
                6f at 288
            })
        }
    }

    // A win's row hops tile by tile once it has turned over, then the sheet.
    var hopping by remember { mutableIntStateOf(-1) }
    val landed = remember { mutableIntStateOf(0) }
    val turning = model.revealing
    LaunchedEffect(turning) { landed.intValue = 0 }

    LaunchedEffect(model.notes) {
        if (model.note != null) {
            delay(NOTE_MS)
            model.clearNote()
        }
    }

    val focus = remember { FocusRequester() }
    LaunchedEffect(Unit) { runCatching { focus.requestFocus() } }

    Box(
        Modifier
            .fillMaxSize()
            .background(tokens.canvas)
            .focusRequester(focus)
            .focusable()
            .onKeyEvent { e -> hardwareKey(model, e) },
    ) {
        Column(Modifier.fillMaxSize().safeDrawingPadding().padding(bottom = 8.dp)) {
            TopBar(
                title = boardTitle(context, v),
                subtitle = stringResource(R.string.code_prefix, v.code),
                onBack = onBack,
                modifier = Modifier.padding(top = 8.dp),
            ) {
                Clock(v, model.viewAt)
            }
            BoxWithConstraints(
                Modifier.fillMaxWidth().weight(1f).padding(top = 20.dp, bottom = 12.dp),
                contentAlignment = Alignment.TopCenter,
            ) {
                val size = tileSize(maxWidth, maxHeight, v.length, v.maxAttempts)
                Grid(
                    v = v,
                    size = size,
                    shakeX = shake.value,
                    revealing = turning,
                    hopping = hopping,
                    onTrim = model::trim,
                    onLanded = {
                        haptics.tick()
                        landed.intValue++
                        if (landed.intValue == v.length) {
                            if (v.status == "won") {
                                haptics.confirm()
                                hopping = turning
                            } else {
                                model.revealed()
                            }
                        }
                    },
                    onHopped = {
                        hopping = -1
                        model.revealed()
                    },
                    moving = moving,
                )
            }
            NoteLine(model, Modifier.fillMaxWidth().height(72.dp))
            Keyboard(
                states = keys.entries.associate { (k, m) -> k.first() to keyStateOf(m) },
                onLetter = { haptics.tick(); model.type(it) },
                onEnter = model::submit,
                onDelete = model::delete,
                enterLabel = stringResource(R.string.key_enter),
                deleteLabel = stringResource(R.string.key_delete),
            )
        }
        ResultSheet(model, onShare = onShare, onHome = onHome)
    }
}

private fun hardwareKey(model: BoardModel, e: androidx.compose.ui.input.key.KeyEvent): Boolean {
    if (e.type != KeyEventType.KeyDown) return false
    when (e.key) {
        Key.Enter, Key.NumPadEnter -> model.submit()
        Key.Backspace -> model.delete()
        else -> {
            val c = e.utf16CodePoint.toChar()
            if (c !in 'a'..'z' && c !in 'A'..'Z') return false
            model.type(c)
        }
    }
    return true
}

/** The largest tile, up to the design's, that fits the grid in the space. */
private fun tileSize(w: Dp, h: Dp, cols: Int, rows: Int): Dp {
    if (cols <= 0 || rows <= 0) return TileSize
    val byWidth = (w - 32.dp - TileGap * (cols - 1)) / cols
    val byHeight = (h - TileGap * (rows - 1)) / rows
    return min(TileSize, min(byWidth, byHeight))
}

@Composable
private fun Clock(v: BoardView, at: Long) {
    var now by remember { mutableLongStateOf(android.os.SystemClock.uptimeMillis()) }
    LaunchedEffect(v.running, at) {
        now = android.os.SystemClock.uptimeMillis()
        while (v.running) {
            delay(1000 - (now - at) % 1000)
            now = android.os.SystemClock.uptimeMillis()
        }
    }
    val ms = v.elapsedMs + if (v.running) now - at else 0
    SurmiseText(clockText(ms), SurmiseTheme.type.clock, SurmiseTheme.tokens.inkDim, maxLines = 1)
}

@Composable
private fun Grid(
    v: BoardView,
    size: Dp,
    shakeX: Float,
    revealing: Int,
    hopping: Int,
    onTrim: (Int) -> Unit,
    onLanded: () -> Unit,
    onHopped: () -> Unit,
    moving: Boolean,
) {
    val done = v.status == "won" || v.status == "lost"
    val current = if (done) -1 else v.rows.size
    Column(verticalArrangement = Arrangement.spacedBy(TileGap), horizontalAlignment = Alignment.CenterHorizontally) {
        for (r in 0 until v.maxAttempts) {
            val row = v.rows.getOrNull(r)
            Row(
                horizontalArrangement = Arrangement.spacedBy(TileGap),
                modifier = if (r == current) Modifier.graphicsLayer { translationX = shakeX * density } else Modifier,
            ) {
                for (c in 0 until v.length) {
                    val (letter, state) = when {
                        row != null -> row.guess.getOrNull(c) to tileStateOf(row.marks.getOrElse(c) { "" })
                        r == current -> when {
                            c < v.typing.length -> v.typing[c] to TileState.Typed
                            c == v.typing.length -> null to TileState.Cursor
                            else -> null to TileState.Ready
                        }
                        else -> null to TileState.Empty
                    }
                    HopBox(hop = r == hopping, index = c, last = c == v.length - 1, moving = moving, onDone = onHopped) {
                        Tile(
                            letter = letter,
                            state = state,
                            size = size,
                            flipDelayMs = if (r == revealing) c * FLIP_STAGGER_MS else 0,
                            onFlipped = { if (r == revealing) onLanded() },
                            modifier = if (r == current && c < v.typing.length) {
                                Modifier.clickable(
                                    interactionSource = remember { MutableInteractionSource() },
                                    indication = null,
                                ) { onTrim(c) }
                            } else {
                                Modifier
                            },
                        )
                    }
                }
            }
        }
    }
}

/** A tile's lift in a win's hop: up and back, after its neighbours on the left. */
@Composable
private fun HopBox(hop: Boolean, index: Int, last: Boolean, moving: Boolean, onDone: () -> Unit, content: @Composable () -> Unit) {
    val y = remember { Animatable(0f) }
    LaunchedEffect(hop) {
        if (!hop) return@LaunchedEffect
        if (moving) {
            y.animateTo(0f, keyframes {
                durationMillis = HOP_MS
                delayMillis = index * HOP_STAGGER_MS
                -10f at HOP_MS * 4 / 10
            })
        }
        if (last) onDone()
    }
    Box(Modifier.graphicsLayer { translationY = y.value * density }) { content() }
}

/** The line between the grid and the keyboard: a refusal, or the done line. */
@Composable
private fun NoteLine(model: BoardModel, modifier: Modifier) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    val context = LocalContext.current
    val v = model.view
    Box(modifier, contentAlignment = Alignment.Center) {
        val note = model.note
        when {
            note != null -> SurmiseText(
                refusalText(context, note, v.length),
                type.body,
                tokens.ink,
                modifier = Modifier
                    .background(tokens.card, CapsuleShape)
                    .border(1.dp, tokens.highlight, CapsuleShape)
                    .padding(horizontal = 14.dp, vertical = 8.dp),
            )
            model.done && model.revealing < 0 && !model.sheet -> Row(
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                CapsuleButton(stringResource(R.string.see_result), model::openResult)
                if (model.result?.next == "random") {
                    CapsuleButton(stringResource(R.string.play_again), model::next, style = CapsuleStyle.Secondary)
                }
            }
            else -> Spacer(Modifier)
        }
    }
}
