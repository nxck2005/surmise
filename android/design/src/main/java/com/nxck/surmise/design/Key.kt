package com.nxck.surmise.design

import androidx.compose.foundation.background
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsPressedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.requiredSize
import androidx.compose.foundation.clickable
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.drawOutline
import androidx.compose.ui.graphics.drawscope.translate
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.unit.dp
import androidx.compose.ui.zIndex

/** What a key shows: its own face, or the best mark its letter has earned. */
enum class KeyState {
    /** Not played yet. */
    Face,

    /** enter and delete. */
    Special,

    /** Played and not in the answer. */
    Spent,

    Correct,
    Present,
}

val KeyHeight = 56.dp
private val KeyGap = 5.dp
private val RowGap = 8.dp
private val SidePadding = 6.dp

// enter and delete are this many letter keys wide (54 against 33).
private const val SPECIAL_WEIGHT = 54f / 33f

/**
 * One key. A pressed letter key raises a balloon above itself with the
 * letter at tile size, so a thumb never hides what it typed.
 */
@Composable
fun Key(
    state: KeyState,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    letter: Char? = null,
    description: String? = null,
    content: @Composable () -> Unit = {},
) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    val source = remember { MutableInteractionSource() }
    val pressed by source.collectIsPressedAsState()
    val (fill, ink) = when (state) {
        KeyState.Face -> tokens.keyFace to tokens.keyText
        KeyState.Special -> tokens.keySpecial to tokens.keyText
        KeyState.Spent -> tokens.keySpent to tokens.keySpentText
        KeyState.Correct -> tokens.tiles.correct.fill to tokens.tiles.correct.letter
        KeyState.Present -> tokens.tiles.present.fill to tokens.tiles.present.letter
    }
    val shape = SquircleShape(Radii.key)
    // An unplayed key stands on a one-pixel ledge, like a real keycap.
    val ledge = state == KeyState.Face || state == KeyState.Special
    Box(
        modifier = modifier
            .height(KeyHeight)
            .zIndex(if (pressed) 1f else 0f)
            .drawBehind {
                if (ledge) {
                    translate(top = 1.dp.toPx()) {
                        drawOutline(
                            shape.createOutline(size, layoutDirection, this),
                            tokens.shadow.copy(alpha = if (tokens.dark) 0.6f else 0.18f),
                        )
                    }
                }
            }
            .background(fill, shape)
            .clickable(
                interactionSource = source,
                indication = null,
                role = Role.Button,
                onClickLabel = description,
                onClick = onClick,
            ),
        contentAlignment = Alignment.Center,
    ) {
        if (letter != null) {
            SurmiseText(letter.uppercaseChar().toString(), type.key, ink)
        } else {
            content()
        }
        if (pressed && letter != null) {
            Balloon(letter)
        }
    }
}

@Composable
private fun Balloon(letter: Char) {
    val tokens = SurmiseTheme.tokens
    val shape = SquircleShape(12.dp)
    val fill = if (tokens.dark) tokens.chartNeutral else tokens.keyFace
    Box(
        modifier = Modifier
            .offset(y = -(KeyHeight + 6.dp))
            .requiredSize(48.dp, KeyHeight)
            .shadow(12.dp, shape, ambientColor = tokens.shadow, spotColor = tokens.shadow)
            .background(fill, shape),
        contentAlignment = Alignment.Center,
    ) {
        SurmiseText(letter.uppercaseChar().toString(), SurmiseTheme.type.tile, tokens.keyText)
    }
}

private val Rows = listOf("qwertyuiop", "asdfghjkl", "zxcvbnm")

/**
 * The qwerty keyboard: three rows, with enter on the left of the last and
 * delete on its right. [states] holds the best mark each letter has earned;
 * a letter missing from it is unplayed. Keys share the width by weight, so
 * the keyboard fills any phone without a fixed key size.
 */
@Composable
fun Keyboard(
    states: Map<Char, KeyState>,
    onLetter: (Char) -> Unit,
    onEnter: () -> Unit,
    onDelete: () -> Unit,
    modifier: Modifier = Modifier,
    enterLabel: String = "enter",
    deleteLabel: String = "delete",
) {
    val tokens = SurmiseTheme.tokens
    Column(
        modifier = modifier.fillMaxWidth().padding(horizontal = SidePadding),
        verticalArrangement = Arrangement.spacedBy(RowGap),
    ) {
        KeyRow {
            Letters(Rows[0], states, onLetter)
        }
        KeyRow {
            // The middle row is nine keys across ten's width, so it is
            // inset by half a key on each side.
            Spacer(Modifier.weight(0.5f))
            Letters(Rows[1], states, onLetter)
            Spacer(Modifier.weight(0.5f))
        }
        KeyRow {
            Key(KeyState.Special, onEnter, Modifier.weight(SPECIAL_WEIGHT), description = enterLabel) {
                SurmiseText(enterLabel, SurmiseTheme.type.keySpecial, tokens.keyText)
            }
            Letters(Rows[2], states, onLetter)
            Key(KeyState.Special, onDelete, Modifier.weight(SPECIAL_WEIGHT), description = deleteLabel) {
                Icon(SurmiseIcons.Backspace, tokens.keyText, size = 22.dp, description = deleteLabel)
            }
        }
    }
}

@Composable
private fun KeyRow(content: @Composable RowScope.() -> Unit) {
    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(KeyGap), content = content)
}

@Composable
private fun RowScope.Letters(row: String, states: Map<Char, KeyState>, onLetter: (Char) -> Unit) {
    for (c in row) {
        Key(states[c] ?: KeyState.Face, { onLetter(c) }, Modifier.weight(1f), letter = c)
    }
}
