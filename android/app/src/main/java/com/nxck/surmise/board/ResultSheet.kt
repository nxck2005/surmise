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
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.nxck.surmise.R
import com.nxck.surmise.boardTitle
import com.nxck.surmise.clockText
import com.nxck.surmise.core.gen.ProfileView
import com.nxck.surmise.core.gen.ResultView
import com.nxck.surmise.design.CapsuleButton
import com.nxck.surmise.design.CapsuleStyle
import com.nxck.surmise.design.Sheet
import com.nxck.surmise.design.SquircleShape
import com.nxck.surmise.design.SurmiseText
import com.nxck.surmise.design.SurmiseTheme
import kotlin.math.roundToInt

/** The debrief over a finished board: the score, the record so far, and what next. */
@Composable
fun ResultSheet(model: BoardModel, onShare: (String) -> Unit, onHome: () -> Unit) {
    val r = model.result
    Sheet(visible = model.sheet && r != null, onDismiss = { model.sheet = false }) {
        if (r != null) {
            ResultBody(
                r = r,
                profile = model.profile,
                onShare = { onShare(r.share) },
                onReview = { model.sheet = false },
                onNext = model::next,
                onHome = onHome,
            )
        }
    }
}

@Composable
fun ResultBody(
    r: ResultView,
    profile: ProfileView?,
    onShare: () -> Unit,
    onReview: () -> Unit,
    onNext: () -> Unit,
    onHome: () -> Unit,
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

        if (profile != null) {
            Stats(profile, b.length, Modifier.padding(top = 20.dp))
            SurmiseText(stringResource(R.string.guesses), type.caption, tokens.inkDim, Modifier.padding(top = 18.dp))
            Distribution(profile.distribution, if (won) b.rows.size else 0, Modifier.padding(top = 8.dp))
        }

        Spacer(Modifier.height(24.dp))
        CapsuleButton(stringResource(R.string.share), onShare, Modifier.fillMaxWidth())
        Row(Modifier.fillMaxWidth().padding(top = 10.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            val next = when (r.next) {
                "random" -> R.string.next_puzzle
                "daily" -> R.string.next_daily
                else -> R.string.home
            }
            CapsuleButton(stringResource(R.string.review), onReview, Modifier.weight(1f), style = CapsuleStyle.Secondary)
            if (r.next == "random" || r.next == "daily") {
                CapsuleButton(stringResource(next), onNext, Modifier.weight(1f), style = CapsuleStyle.Secondary)
            }
            CapsuleButton(stringResource(R.string.home), onHome, Modifier.weight(1f), style = CapsuleStyle.Secondary)
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

/** Played, won and streak for the board's length, between two hairlines. */
@Composable
private fun Stats(p: ProfileView, length: Int, modifier: Modifier) {
    val tokens = SurmiseTheme.tokens
    val mode = p.modes.firstOrNull { it.length == length }
    val cells = listOf(
        (mode?.played ?: 0).toString() to stringResource(R.string.stat_played),
        "${((mode?.winRate ?: 0.0) * 100).roundToInt()}%" to stringResource(R.string.stat_won),
        p.currentStreak.toString() to stringResource(R.string.stat_streak),
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

/** Wins by guesses, one bar each; this board's bar in the chart's highlight. */
@Composable
private fun Distribution(counts: List<Int>, mine: Int, modifier: Modifier) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    val top = (counts.maxOrNull() ?: 0).coerceAtLeast(1)
    Column(modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(6.dp)) {
        counts.forEachIndexed { i, n ->
            val on = i + 1 == mine
            Row(Modifier.fillMaxWidth().height(16.dp), verticalAlignment = Alignment.CenterVertically) {
                SurmiseText("${i + 1}", type.code, tokens.inkMuted, Modifier.width(18.dp))
                Box(Modifier.weight(1f)) {
                    Box(
                        Modifier
                            .fillMaxWidth(n.toFloat() / top)
                            .height(16.dp)
                            .background(if (on) tokens.chartHighlight else tokens.chartNeutral, SquircleShape(4.dp)),
                    )
                }
                SurmiseText("$n", type.footnote, if (on) tokens.ink else tokens.inkMuted, Modifier.width(28.dp), align = TextAlign.End)
            }
        }
    }
}
