package com.nxck.surmise

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import com.nxck.surmise.design.KeyState
import com.nxck.surmise.design.Radii
import com.nxck.surmise.design.SquircleShape
import com.nxck.surmise.design.SurmiseTheme
import com.nxck.surmise.design.SurmiseTokens
import com.nxck.surmise.design.TileState

// The engine names marks as strings ("correct", "present", "absent"); these
// turn them into the design module's states and fills.

fun tileStateOf(mark: String): TileState = when (mark) {
    "correct" -> TileState.Correct
    "present" -> TileState.Present
    else -> TileState.Absent
}

fun keyStateOf(mark: String): KeyState = when (mark) {
    "correct" -> KeyState.Correct
    "present" -> KeyState.Present
    else -> KeyState.Spent
}

/** The fill of a scored mini tile, or null for one not played yet. */
fun markFill(tokens: SurmiseTokens, mark: String?): Color? = when (mark) {
    "correct" -> tokens.tiles.correct.fill
    "present" -> tokens.tiles.present.fill
    "absent" -> tokens.tiles.absent.fill
    else -> null
}

/**
 * A board drawn as marks only, no letters: the daily card's three boards and
 * a history row. [marks] is one list per guess; rows past them are outlines.
 */
@Composable
fun MiniBoard(
    length: Int,
    rows: Int,
    marks: List<List<String>>,
    modifier: Modifier = Modifier,
    cell: Dp = 10.dp,
    gap: Dp = 2.dp,
) {
    val tokens = SurmiseTheme.tokens
    val shape = SquircleShape(Radii.miniTile)
    Column(modifier, verticalArrangement = Arrangement.spacedBy(gap)) {
        for (r in 0 until rows) {
            Row(horizontalArrangement = Arrangement.spacedBy(gap)) {
                for (c in 0 until length) {
                    val fill = markFill(tokens, marks.getOrNull(r)?.getOrNull(c))
                    val m = Modifier.size(cell).clip(shape)
                    if (fill != null) {
                        Box(m.background(fill))
                    } else {
                        Box(m.border(1.dp, tokens.tileEmpty, shape))
                    }
                }
            }
        }
    }
}
