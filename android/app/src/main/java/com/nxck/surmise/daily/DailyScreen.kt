package com.nxck.surmise.daily

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import com.nxck.surmise.BackButton
import com.nxck.surmise.MiniBoard
import com.nxck.surmise.R
import com.nxck.surmise.clockText
import com.nxck.surmise.core.gen.DailyRow
import com.nxck.surmise.core.gen.DailyView
import com.nxck.surmise.countText
import com.nxck.surmise.design.CapsuleButton
import com.nxck.surmise.design.CapsuleShape
import com.nxck.surmise.design.CapsuleStyle
import com.nxck.surmise.design.Card
import com.nxck.surmise.design.MeshGlow
import com.nxck.surmise.design.Radii
import com.nxck.surmise.design.SquircleShape
import com.nxck.surmise.design.SurmiseText
import com.nxck.surmise.design.SurmiseTheme
import kotlinx.coroutines.delay

/** The daily screen: the day's three boards, the trio across them, and its share. */
@Composable
fun DailyScreen(
    daily: DailyView?,
    date: String,
    playedToday: Int,
    onBack: () -> Unit,
    onOpen: (length: Int) -> Unit,
    onShare: (String) -> Unit,
    now: () -> Long = System::currentTimeMillis,
) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    Column(
        Modifier
            .fillMaxSize()
            .background(tokens.canvas)
            .safeDrawingPadding()
            .verticalScroll(rememberScrollState())
            .padding(bottom = 24.dp),
    ) {
        Row(
            Modifier.fillMaxWidth().height(44.dp).padding(horizontal = 16.dp).padding(top = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            BackButton(onBack)
            Box(Modifier.weight(1f), contentAlignment = Alignment.CenterEnd) {
                if (daily != null && daily.resetsAtMs > 0) NextIn(daily.resetsAtMs, now)
            }
        }
        Column(Modifier.padding(start = 20.dp, end = 20.dp, top = 20.dp)) {
            SurmiseText(stringResource(R.string.daily), type.display, tokens.ink)
            SurmiseText(date, type.caption, tokens.inkMuted, Modifier.padding(top = 2.dp))
        }
        if (daily == null) return@Column
        TrioCard(daily, onShare, Modifier.padding(start = 16.dp, end = 16.dp, top = 20.dp))
        Column(
            Modifier.padding(start = 16.dp, end = 16.dp, top = 12.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            for (row in daily.rows) ModeRow(row) { onOpen(row.length) }
        }
        if (playedToday > 0) {
            SurmiseText(
                stringResource(R.string.daily_played_all, countText(playedToday)),
                type.footnote,
                tokens.inkMuted,
                Modifier.fillMaxWidth().padding(top = 16.dp),
                align = androidx.compose.ui.text.style.TextAlign.Center,
            )
        }
    }
}

/** "next daily in 9h 12m", ticking once a minute. */
@Composable
private fun NextIn(resetsAt: Long, now: () -> Long) {
    var t by remember { mutableLongStateOf(now()) }
    LaunchedEffect(resetsAt) {
        while (true) {
            t = now()
            delay(60_000 - t % 60_000)
        }
    }
    val left = ((resetsAt - t).coerceAtLeast(0) + 59_999) / 60_000
    val text = if (left >= 60) {
        stringResource(R.string.daily_next_hm, left / 60, left % 60)
    } else {
        stringResource(R.string.daily_next_m, left)
    }
    SurmiseText(text, SurmiseTheme.type.caption, SurmiseTheme.tokens.inkMuted)
}

@Composable
private fun TrioCard(d: DailyView, onShare: (String) -> Unit, modifier: Modifier) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    MeshGlow(modifier.fillMaxWidth().clip(SquircleShape(Radii.hero))) {
        Column(Modifier.padding(20.dp)) {
            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.Bottom) {
                SurmiseText(stringResource(R.string.daily_title), type.title, tokens.ink, Modifier.weight(1f))
                SurmiseText(stringResource(R.string.daily_done, d.trio.done, d.trio.of), type.caption, tokens.inkDim)
            }
            Row(Modifier.padding(top = 14.dp), horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                for (row in d.rows) {
                    val fill = when (row.state) {
                        "won" -> tokens.tiles.correct.fill
                        "lost" -> tokens.inkMuted
                        "playing" -> tokens.action
                        else -> tokens.ink.copy(alpha = 0.12f)
                    }
                    Box(Modifier.weight(1f).height(6.dp).background(fill, CapsuleShape))
                }
            }
            if (d.trio.complete && d.share.isNotEmpty()) {
                Row(Modifier.fillMaxWidth().padding(top = 16.dp), verticalAlignment = Alignment.CenterVertically) {
                    SurmiseText(
                        pluralStringResource(R.plurals.trio_summary, d.trio.guesses, d.trio.guesses, clockText(d.trio.elapsedMs)),
                        type.body,
                        tokens.inkDim,
                        Modifier.weight(1f),
                    )
                    CapsuleButton(stringResource(R.string.share), { onShare(d.share) })
                }
            } else {
                SurmiseText(stringResource(R.string.trio_hint), type.caption, tokens.inkDim, Modifier.padding(top = 14.dp))
            }
        }
    }
}

@Composable
private fun ModeRow(row: DailyRow, onOpen: () -> Unit) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    Card(Modifier.fillMaxWidth(), padding = PaddingValues(14.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Box(
                Modifier
                    .size(72.dp)
                    .background(tokens.canvas.copy(alpha = 0.4f), SquircleShape(Radii.badge))
                    .border(1.dp, tokens.highlight, SquircleShape(Radii.badge)),
                contentAlignment = Alignment.Center,
            ) {
                MiniBoard(row.length, row.maxAttempts, row.marks, cell = 7.dp, gap = 1.5.dp)
            }
            Column(Modifier.weight(1f).padding(start = 14.dp)) {
                SurmiseText(stringResource(R.string.letters, row.length), type.section, tokens.ink)
                val (note, color) = when (row.state) {
                    "won" -> stringResource(R.string.daily_solved_time, row.attempts, clockText(row.elapsedMs)) to tokens.accentText
                    "lost" -> stringResource(R.string.daily_note_lost) to tokens.inkDim
                    "playing" -> pluralStringResource(R.plurals.guesses_in, row.attempts, row.attempts) to tokens.action
                    "spent" -> stringResource(R.string.daily_note_spent) to tokens.inkMuted
                    else -> stringResource(R.string.daily_note_new) to tokens.inkMuted
                }
                SurmiseText(note, type.caption, color, Modifier.padding(top = 2.dp))
            }
            when (row.state) {
                "playing" -> CapsuleButton(stringResource(R.string.daily_action_continue), onOpen)
                "new" -> CapsuleButton(stringResource(R.string.daily_action_play), onOpen, style = CapsuleStyle.Secondary)
                "won", "lost" -> CapsuleButton(stringResource(R.string.review), onOpen, style = CapsuleStyle.Secondary)
                else -> Box(Modifier.width(1.dp))
            }
        }
    }
}
