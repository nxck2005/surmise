package com.nxck.surmise.home

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import com.nxck.surmise.MiniBoard
import com.nxck.surmise.R
import com.nxck.surmise.countText
import com.nxck.surmise.core.gen.DailyRow
import com.nxck.surmise.core.gen.DailyView
import com.nxck.surmise.core.gen.HistoryItem
import com.nxck.surmise.design.CapsuleButton
import com.nxck.surmise.design.CapsuleShape
import com.nxck.surmise.design.Card
import com.nxck.surmise.design.Icon
import com.nxck.surmise.design.MeshGlow
import com.nxck.surmise.design.Radii
import com.nxck.surmise.design.SquircleShape
import com.nxck.surmise.design.SurmiseIcons
import com.nxck.surmise.design.SurmiseText
import com.nxck.surmise.design.SurmiseTheme
import com.nxck.surmise.design.TabBarHeight
import com.nxck.surmise.design.pressable
import com.nxck.surmise.homeDate
import com.nxck.surmise.refusalText
import kotlinx.coroutines.delay

/** Everything the play tab shows, read from the engine by the app model. */
data class HomeState(
    val date: String,
    val name: String,
    val daily: DailyView?,
    /** The newest random board in play for 4, 5 and 6 letters, if any. */
    val inPlay: Map<Int, HistoryItem>,
    /** Today's finished dailies across everyone, or 0 when not shown. */
    val playedToday: Int,
    val note: Throwable? = null,
    /** Whether rooms are on offer (online, and the server has them). */
    val rooms: Boolean = false,
    /** A room whose round of ours is still running, found at startup, or "". */
    val running: String = "",
    /** Everyone's finished boards of today's daily, by length; empty when not shown. */
    val dailyCounts: Map<Int, Int> = emptyMap(),
)

/** What the play tab's taps do. */
data class HomeActions(
    val onDaily: (length: Int) -> Unit,
    /** Resumes the length's board in play, or starts one. */
    val onPlay: (length: Int) -> Unit,
    /** Starts a new board of the length, leaving the one in play in the history. */
    val onNew: (length: Int) -> Unit = {},
    val onSprint: () -> Unit = {},
    val onHowTo: () -> Unit = {},
    val onSettings: () -> Unit = {},
    val onShare: (String) -> Unit = {},
    val onRoom: () -> Unit = {},
)

@Composable
fun HomeScreen(state: HomeState, actions: HomeActions, modifier: Modifier = Modifier, now: () -> Long = System::currentTimeMillis) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    val context = LocalContext.current
    Column(
        modifier
            .verticalScroll(rememberScrollState())
            .statusBarsPadding()
            .padding(bottom = TabBarHeight + 48.dp),
    ) {
        Header(state, actions)
        if (state.running.isNotEmpty()) RunningCard(state.running, actions.onRoom)
        DailyCard(state, actions, now, Modifier.padding(start = 16.dp, end = 16.dp, top = 18.dp))
        if (state.note != null) {
            SurmiseText(
                refusalText(context, state.note),
                type.caption,
                tokens.danger,
                Modifier.padding(start = 20.dp, end = 20.dp, top = 12.dp),
            )
        }
        Row(Modifier.fillMaxWidth().padding(start = 20.dp, end = 20.dp, top = 22.dp, bottom = 10.dp), verticalAlignment = Alignment.Bottom) {
            SurmiseText(stringResource(R.string.section_practice), type.section, tokens.ink, Modifier.weight(1f))
            SurmiseText(stringResource(R.string.practice_line), type.footnote, tokens.inkMuted)
        }
        Row(Modifier.padding(horizontal = 16.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            for (n in 4..6) {
                PlayCard(n, state.inPlay[n], { actions.onPlay(n) }, { actions.onNew(n) }, Modifier.weight(1f))
            }
        }
        SprintCard(actions.onSprint, Modifier.padding(start = 16.dp, end = 16.dp, top = 8.dp))
    }
}

/** A round of ours still running in a room, and the way back in. */
@Composable
private fun RunningCard(code: String, onClick: () -> Unit) {
    val tokens = SurmiseTheme.tokens
    Card(Modifier.fillMaxWidth().padding(start = 16.dp, end = 16.dp, top = 18.dp).pressable(onClick = onClick), padding = PaddingValues(16.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Icon(SurmiseIcons.Rooms, tokens.accentText, size = 22.dp)
            SurmiseText(stringResource(R.string.room_still_running, code), SurmiseTheme.type.body, tokens.ink, Modifier.weight(1f).padding(start = 12.dp))
            SurmiseText(stringResource(R.string.room_rejoin), SurmiseTheme.type.caption, tokens.accentText)
        }
    }
}

/** Sprint, one full-width row: its icon, its name and what it is. */
@Composable
private fun SprintCard(onClick: () -> Unit, modifier: Modifier) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    Card(modifier.fillMaxWidth().height(64.dp).pressable(onClick = onClick), padding = PaddingValues(horizontal = 16.dp)) {
        Row(Modifier.fillMaxSize(), verticalAlignment = Alignment.CenterVertically) {
            Icon(SurmiseIcons.Sprint, tokens.accentText, size = 22.dp)
            Column(Modifier.weight(1f).padding(start = 14.dp)) {
                SurmiseText(stringResource(R.string.sprint), type.body.copy(fontWeight = type.section.fontWeight), tokens.ink, maxLines = 1)
                SurmiseText(stringResource(R.string.way_sprint_long), type.footnote, tokens.inkDim, maxLines = 1)
            }
        }
    }
}

/** A round button in the header: how to play, or settings. */
@Composable
private fun RoundButton(label: String, onClick: () -> Unit, content: @Composable () -> Unit) {
    Box(
        Modifier
            .size(44.dp)
            .pressable(onClick = onClick)
            .background(SurmiseTheme.tokens.card, CapsuleShape)
            .semantics { contentDescription = label },
        contentAlignment = Alignment.Center,
    ) { content() }
}

@Composable
private fun Header(state: HomeState, actions: HomeActions) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    Row(
        Modifier.fillMaxWidth().padding(start = 20.dp, end = 20.dp, top = 18.dp),
        verticalAlignment = Alignment.Bottom,
    ) {
        Column(Modifier.weight(1f)) {
            SurmiseText(state.date, type.caption, tokens.inkMuted)
            SurmiseText(stringResource(R.string.app_name), type.display, tokens.ink, Modifier.padding(top = 2.dp))
        }
        Row(Modifier.padding(bottom = 4.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            RoundButton(stringResource(R.string.how_to_play_label), actions.onHowTo) {
                SurmiseText("?", type.headline.copy(fontWeight = type.section.fontWeight), tokens.ink)
            }
            RoundButton(stringResource(R.string.tab_settings), actions.onSettings) {
                Icon(SurmiseIcons.Settings, tokens.ink, size = 20.dp)
            }
        }
    }
}

private fun DailyRow.done(): Boolean = state == "won" || state == "lost"

/**
 * Today's three, the one card with colour behind it: each length drawn as
 * its real mini board, the one in play edged in orange, a solved one
 * checked. The card itself is not a button: each board opens its own
 * daily, and the button the next one, or the share once all three are over.
 */
@Composable
private fun DailyCard(state: HomeState, actions: HomeActions, now: () -> Long, modifier: Modifier) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    val d = state.daily
    val rows = d?.rows.orEmpty()
    MeshGlow(modifier.fillMaxWidth().clip(SquircleShape(Radii.hero))) {
        Column(Modifier.padding(start = 20.dp, end = 20.dp, top = 18.dp, bottom = 16.dp)) {
            Row(Modifier.fillMaxWidth()) {
                SurmiseText(stringResource(R.string.daily), type.caption, tokens.inkDim, Modifier.weight(1f))
                if (d != null) {
                    SurmiseText(stringResource(R.string.daily_done, d.trio.done, d.trio.of), type.caption, tokens.inkDim)
                }
            }
            SurmiseText(stringResource(R.string.daily_title), type.title, tokens.ink, Modifier.padding(top = 2.dp))
            Row(Modifier.fillMaxWidth().padding(top = 14.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                for (row in rows) {
                    val played = if (state.dailyCounts.isEmpty()) null else state.dailyCounts[row.length] ?: 0
                    DailyBoard(row, played, { actions.onDaily(row.length) }, Modifier.weight(1f))
                }
            }
            val next = rows.firstOrNull { it.state == "playing" } ?: rows.firstOrNull { it.state == "new" }
            Row(Modifier.fillMaxWidth().padding(top = 16.dp), verticalAlignment = Alignment.CenterVertically) {
                Box(Modifier.weight(1f)) {
                    when {
                        next == null && d != null && d.resetsAtMs > 0 -> NextDailyIn(d.resetsAtMs, now)
                        state.playedToday > 0 -> SurmiseText(
                            stringResource(R.string.daily_played_today, countText(state.playedToday)),
                            type.footnote,
                            tokens.inkDim,
                        )
                    }
                }
                if (next != null) {
                    val label = if (next.state == "playing") R.string.daily_continue else R.string.daily_start
                    CapsuleButton(stringResource(label, next.length), { actions.onDaily(next.length) })
                } else if (d != null && d.trio.complete && d.share.isNotEmpty()) {
                    CapsuleButton(stringResource(R.string.share), { actions.onShare(d.share) })
                }
            }
        }
    }
}

/** "next daily in 9h 12m", ticking once a minute. */
@Composable
private fun NextDailyIn(resetsAt: Long, now: () -> Long) {
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
    SurmiseText(text, SurmiseTheme.type.footnote, SurmiseTheme.tokens.inkDim)
}

@Composable
private fun DailyBoard(row: DailyRow, played: Int?, onClick: () -> Unit, modifier: Modifier) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    val playing = row.state == "playing"
    val shape = SquircleShape(Radii.inner)
    val face = if (playing) {
        Modifier.background(tokens.action.copy(alpha = 0.10f), shape).border(1.5.dp, tokens.action, shape)
    } else {
        Modifier.background(tokens.canvas.copy(alpha = 0.35f), shape).border(1.dp, tokens.highlight, shape)
    }
    Box(modifier.height(if (played != null) 166.dp else 148.dp).pressable(onClick = onClick).then(face)) {
        Column(
            Modifier.fillMaxWidth().padding(top = 12.dp, bottom = 10.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Box(Modifier.height(86.dp), contentAlignment = Alignment.Center) {
                MiniBoard(row.length, row.maxAttempts, row.marks)
            }
            SurmiseText(stringResource(R.string.letters, row.length), type.caption.copy(fontWeight = type.section.fontWeight), tokens.ink, Modifier.padding(top = 8.dp))
            val (note, color) = when (row.state) {
                "won" -> pluralStringResource(R.plurals.daily_note_solved, row.attempts, row.attempts) to tokens.inkDim
                "lost" -> stringResource(R.string.daily_note_lost) to tokens.inkDim
                "playing" -> pluralStringResource(R.plurals.guesses_in, row.attempts, row.attempts) to tokens.accentText
                "spent" -> stringResource(R.string.daily_note_spent) to tokens.inkMuted
                else -> stringResource(R.string.daily_note_new) to tokens.inkMuted
            }
            SurmiseText(note, type.footnote, color, maxLines = 1)
            if (played != null) SurmiseText(stringResource(R.string.daily_board_count, countText(played)), type.footnote, tokens.inkMuted, maxLines = 1)
        }
        if (row.state == "won") {
            Box(
                Modifier.align(Alignment.TopEnd).padding(8.dp).size(18.dp).background(tokens.tiles.correct.fill, CapsuleShape),
                contentAlignment = Alignment.Center,
            ) {
                Icon(SurmiseIcons.Check, tokens.tiles.correct.letter, size = 10.dp)
            }
        }
    }
}

/**
 * One length to practise. With a board in play the card has two targets:
 * the top resumes it, and a band at the foot starts a new word. With none,
 * the whole card starts a new word.
 */
@Composable
private fun PlayCard(length: Int, inPlay: HistoryItem?, onPlay: () -> Unit, onNew: () -> Unit, modifier: Modifier) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    val shape = SquircleShape(Radii.card)
    val resumeLabel = inPlay?.let { pluralStringResource(R.plurals.practice_resume_label, it.attempts, length, it.attempts, length + 1) }
    val newLabel = stringResource(R.string.practice_new_label, length)
    Column(modifier.height(128.dp).clip(shape).background(tokens.card, shape)) {
        Column(
            Modifier
                .fillMaxWidth()
                .weight(1f)
                .pressable(onClick = onPlay)
                .semantics { contentDescription = resumeLabel ?: newLabel }
                .padding(horizontal = 14.dp, vertical = 12.dp),
        ) {
            Row(horizontalArrangement = Arrangement.spacedBy(3.dp)) {
                repeat(length) {
                    Box(Modifier.size(10.dp).border(1.5.dp, tokens.tileEmpty, SquircleShape(Radii.miniTile)))
                }
            }
            Spacer(Modifier.weight(1f))
            SurmiseText(stringResource(R.string.letters, length), type.body.copy(fontWeight = type.section.fontWeight), tokens.ink, maxLines = 1)
            if (inPlay != null) {
                SurmiseText(stringResource(R.string.practice_resume, inPlay.attempts, length + 1), type.footnote, tokens.accentText, maxLines = 1)
            } else {
                SurmiseText(stringResource(R.string.practice_new), type.footnote, tokens.inkMuted, maxLines = 1)
            }
        }
        if (inPlay != null) {
            Box(Modifier.fillMaxWidth().height(1.dp).background(tokens.highlight))
            Row(
                Modifier
                    .fillMaxWidth()
                    .height(40.dp)
                    .pressable(onClick = onNew)
                    .semantics { contentDescription = newLabel },
                horizontalArrangement = Arrangement.Center,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                SurmiseText("+ " + stringResource(R.string.practice_new), type.footnote.copy(fontWeight = type.section.fontWeight), tokens.ink, maxLines = 1)
            }
        }
    }
}

/** Home's state from today's date and the model's reads. */
fun homeState(
    name: String,
    daily: DailyView?,
    history: List<HistoryItem>,
    playedToday: Int,
    note: Throwable?,
    rooms: Boolean = false,
    running: String = "",
    dailyCounts: Map<Int, Int> = emptyMap(),
): HomeState =
    HomeState(
        date = homeDate(),
        name = name,
        daily = daily,
        inPlay = (4..6).mapNotNull { n ->
            history.firstOrNull { it.kind == "random" && it.length == n && it.status == "in_progress" }?.let { n to it }
        }.toMap(),
        playedToday = playedToday,
        note = note,
        rooms = rooms,
        running = running,
        dailyCounts = dailyCounts,
    )
