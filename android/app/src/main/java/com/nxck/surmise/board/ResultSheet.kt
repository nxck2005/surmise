package com.nxck.surmise.board

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import com.nxck.surmise.R
import com.nxck.surmise.boardTitle
import com.nxck.surmise.clockText
import com.nxck.surmise.core.gen.DailyView
import com.nxck.surmise.core.gen.ProfileView
import com.nxck.surmise.core.gen.ResultView
import com.nxck.surmise.design.CapsuleButton
import com.nxck.surmise.design.CapsuleStyle
import com.nxck.surmise.design.Sheet
import com.nxck.surmise.design.SurmiseText
import com.nxck.surmise.design.SurmiseTheme
import java.util.Locale
import kotlin.math.roundToInt

/** The result sheet's second button: where the player goes after this board. */
data class NextAction(val label: String, val onClick: () -> Unit)

/**
 * The daily to play after the [length] one: today's first that is not over,
 * in 4, 5, 6 order. The [length] one is skipped, as the view read before it
 * finished may still call it in play.
 */
fun nextDailyLength(daily: DailyView?, length: Int): Int? =
    daily?.rows?.sortedBy { it.length }?.firstOrNull { it.length != length && (it.state == "new" || it.state == "playing") }?.length

/** The debrief over a finished board: the score, the record so far, and what next. */
@Composable
fun ResultSheet(model: BoardModel, onShare: (String) -> Unit, onHome: () -> Unit, daily: DailyView? = null) {
    val r = model.result
    Sheet(visible = model.sheet && r != null, onDismiss = { model.sheet = false }) {
        if (r != null) {
            ResultBody(
                r = r,
                profile = model.profile,
                onShare = { onShare(r.share) },
                onDone = onHome,
                next = nextAction(r, daily, model::next, onShare),
            )
        }
    }
}

/**
 * Next for a random board is a new word; for a daily, the next daily of
 * today, or once all three are over, the three as one share. Other kinds
 * have no next.
 */
@Composable
private fun nextAction(r: ResultView, daily: DailyView?, onNext: () -> Unit, onShare: (String) -> Unit): NextAction? = when (r.next) {
    "random" -> NextAction(stringResource(R.string.next_word), onNext)
    "daily" -> {
        val n = nextDailyLength(daily, r.board.length)
        when {
            n != null -> NextAction(stringResource(R.string.next_daily, n), onNext)
            daily != null && daily.trio.complete && daily.share.isNotEmpty() ->
                NextAction(stringResource(R.string.share_all_three)) { onShare(daily.share) }
            else -> null
        }
    }
    else -> null
}

@Composable
fun ResultBody(
    r: ResultView,
    profile: ProfileView?,
    onShare: () -> Unit,
    onDone: () -> Unit,
    next: NextAction?,
) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    val context = LocalContext.current
    val b = r.board
    val won = b.status == "won"

    Column(Modifier.fillMaxWidth().navigationBarsPadding()) {
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.Bottom) {
            SurmiseText(boardTitle(context, b), type.caption, tokens.inkDim, Modifier.weight(1f))
            SurmiseText(stringResource(R.string.code_prefix, b.code), type.code, tokens.inkMuted)
        }
        SurmiseText(
            if (won) stringResource(R.string.solved_in, b.rows.size) else stringResource(R.string.not_this_time),
            type.display,
            tokens.ink,
            Modifier.padding(top = 4.dp),
        )
        SurmiseText(
            subline(r, won, context),
            type.body.copy(fontSize = type.headline.fontSize),
            tokens.inkDim,
            Modifier.padding(top = 6.dp),
        )

        if (profile != null) Stats(profile, b.length, Modifier.padding(top = 20.dp))

        Spacer(Modifier.height(24.dp))
        CapsuleButton(stringResource(R.string.share), onShare, Modifier.fillMaxWidth())
        Row(Modifier.fillMaxWidth().padding(top = 10.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            if (next != null) CapsuleButton(next.label, next.onClick, Modifier.weight(2f), style = CapsuleStyle.Secondary)
            CapsuleButton(stringResource(R.string.result_done), onDone, Modifier.weight(1f), style = CapsuleStyle.Secondary)
        }
    }
}

private fun subline(r: ResultView, won: Boolean, context: android.content.Context): String {
    val b = r.board
    if (!won) return context.getString(R.string.answer_was, b.answer.uppercase())
    val time = clockText(b.elapsedMs)
    val best = when {
        r.fastest && r.fewest -> context.getString(R.string.best_both, b.length)
        r.fastest -> context.getString(R.string.best_fastest, b.length)
        r.fewest -> context.getString(R.string.best_fewest, b.length)
        else -> null
    }
    return if (best != null) "$time, $best" else time
}

/**
 * Played, won and average guesses for the board's length only, between two
 * hairlines. Nothing from the other lengths, so every figure means one thing.
 */
@Composable
private fun Stats(p: ProfileView, length: Int, modifier: Modifier) {
    val tokens = SurmiseTheme.tokens
    val mode = p.modes.firstOrNull { it.length == length }
    val cells = listOf(
        (mode?.played ?: 0).toString() to stringResource(R.string.stat_played),
        "${((mode?.winRate ?: 0.0) * 100).roundToInt()}%" to stringResource(R.string.stat_won),
        String.format(Locale.ROOT, "%.1f", mode?.avgAttempts ?: 0.0) to stringResource(R.string.stat_avg_guesses),
    )
    Column(modifier.fillMaxWidth()) {
        Hairline()
        Row(Modifier.fillMaxWidth().height(64.dp)) {
            cells.forEachIndexed { i, (n, label) ->
                if (i > 0) Box(Modifier.width(1.dp).fillMaxHeight().background(tokens.ink.copy(alpha = 0.08f)))
                Column(
                    Modifier.weight(1f).fillMaxHeight().padding(start = if (i > 0) 16.dp else 0.dp),
                    verticalArrangement = Arrangement.Center,
                ) {
                    SurmiseText(n, SurmiseTheme.type.displaySmall.copy(fontFamily = SurmiseTheme.type.body.fontFamily), tokens.ink)
                    SurmiseText(label, SurmiseTheme.type.footnote, tokens.inkMuted)
                }
            }
        }
        Hairline()
    }
}

@Composable
private fun Hairline() {
    Box(Modifier.fillMaxWidth().height(1.dp).background(SurmiseTheme.tokens.ink.copy(alpha = 0.08f)))
}
