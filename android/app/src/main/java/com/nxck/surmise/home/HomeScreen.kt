package com.nxck.surmise.home

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
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
import com.nxck.surmise.design.CapsuleStyle
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

/** Everything home shows, read from the engine by the app model. */
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
)

/** What home's taps do. */
data class HomeActions(
    val onDaily: (length: Int) -> Unit,
    val onDailyScreen: () -> Unit,
    val onPlay: (length: Int) -> Unit,
    val onRecord: () -> Unit,
    val onWay: (Way) -> Unit = {},
)

/** The other ways to play, in home's sideways row. */
enum class Way { Rooms, Sprint, Challenge, Custom }

@Composable
fun HomeScreen(state: HomeState, actions: HomeActions, modifier: Modifier = Modifier) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    val context = LocalContext.current
    Column(
        modifier
            .verticalScroll(rememberScrollState())
            .statusBarsPadding()
            .padding(bottom = TabBarHeight + 48.dp),
    ) {
        Header(state, actions.onRecord)
        DailyCard(state, actions.onDaily, actions.onDailyScreen, Modifier.padding(start = 16.dp, end = 16.dp, top = 18.dp))
        if (state.note != null) {
            SurmiseText(
                refusalText(context, state.note),
                type.caption,
                tokens.danger,
                Modifier.padding(start = 20.dp, end = 20.dp, top = 12.dp),
            )
        }
        SurmiseText(stringResource(R.string.section_play), type.section, tokens.ink, Modifier.padding(start = 20.dp, top = 22.dp, bottom = 10.dp))
        Row(Modifier.padding(horizontal = 16.dp), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            for (n in 4..6) {
                PlayCard(n, state.inPlay[n], { actions.onPlay(n) }, Modifier.weight(1f))
            }
        }
        SurmiseText(stringResource(R.string.section_more), type.section, tokens.ink, Modifier.padding(start = 20.dp, top = 22.dp, bottom = 10.dp))
        LazyRow(contentPadding = PaddingValues(horizontal = 16.dp), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            items(Way.entries.filter { it != Way.Rooms || state.rooms }) { w -> WayCard(w) { actions.onWay(w) } }
        }
    }
}

/** One other way to play: its icon, its name and what it is, in a line. */
@Composable
private fun WayCard(way: Way, onClick: () -> Unit) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    val (icon, title, line) = when (way) {
        Way.Rooms -> Triple(SurmiseIcons.Rooms, R.string.rooms, R.string.way_rooms)
        Way.Sprint -> Triple(SurmiseIcons.Sprint, R.string.sprint, R.string.way_sprint)
        Way.Challenge -> Triple(SurmiseIcons.Share, R.string.challenge, R.string.way_challenge)
        Way.Custom -> Triple(SurmiseIcons.Custom, R.string.custom, R.string.way_custom)
    }
    Card(Modifier.width(148.dp).height(112.dp).pressable(onClick = onClick), padding = PaddingValues(14.dp)) {
        Column(Modifier.fillMaxWidth()) {
            Icon(icon, tokens.accentText, size = 22.dp)
            Spacer(Modifier.weight(1f))
            SurmiseText(stringResource(title), type.body, tokens.ink, maxLines = 1)
            SurmiseText(stringResource(line), type.footnote, tokens.inkMuted, maxLines = 2)
        }
    }
}

@Composable
private fun Header(state: HomeState, onRecord: () -> Unit) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    val label = stringResource(R.string.home_record)
    Row(
        Modifier.fillMaxWidth().padding(start = 20.dp, end = 20.dp, top = 18.dp),
        verticalAlignment = Alignment.Bottom,
    ) {
        Column(Modifier.weight(1f)) {
            SurmiseText(state.date, type.caption, tokens.inkMuted)
            SurmiseText(stringResource(R.string.app_name), type.display, tokens.ink, Modifier.padding(top = 2.dp))
        }
        Box(
            Modifier
                .padding(bottom = 4.dp)
                .size(36.dp)
                .pressable(onClick = onRecord)
                .background(tokens.card, CapsuleShape)
                .semantics { contentDescription = label },
            contentAlignment = Alignment.Center,
        ) {
            val initial = state.name.trim().firstOrNull()?.lowercaseChar()
            if (initial != null) {
                SurmiseText(initial.toString(), type.displaySmall.copy(fontSize = type.headline.fontSize), tokens.ink)
            } else {
                Icon(SurmiseIcons.Record, tokens.inkDim, size = 16.dp)
            }
        }
    }
}

private fun DailyRow.done(): Boolean = state == "won" || state == "lost"

/**
 * Today's three, the one card with colour behind it: each length drawn as
 * its real mini board, the one in play edged in orange, a solved one checked.
 */
@Composable
private fun DailyCard(state: HomeState, onDaily: (Int) -> Unit, onScreen: () -> Unit, modifier: Modifier) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    val d = state.daily
    val rows = d?.rows.orEmpty()
    MeshGlow(modifier.fillMaxWidth().clip(SquircleShape(Radii.hero)).pressable(onClick = onScreen)) {
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
                    DailyBoard(row, { onDaily(row.length) }, Modifier.weight(1f))
                }
            }
            Row(Modifier.fillMaxWidth().padding(top = 16.dp), verticalAlignment = Alignment.CenterVertically) {
                Box(Modifier.weight(1f)) {
                    if (state.playedToday > 0) {
                        SurmiseText(
                            stringResource(R.string.daily_played_today, countText(state.playedToday)),
                            type.footnote,
                            tokens.inkDim,
                        )
                    }
                }
                val next = rows.firstOrNull { it.state == "playing" } ?: rows.firstOrNull { it.state == "new" }
                if (next != null) {
                    val label = if (next.state == "playing") R.string.daily_continue else R.string.daily_start
                    CapsuleButton(stringResource(label, next.length), { onDaily(next.length) })
                } else if (rows.isNotEmpty()) {
                    CapsuleButton(stringResource(R.string.daily_see), onScreen, style = CapsuleStyle.Secondary)
                }
            }
        }
    }
}

@Composable
private fun DailyBoard(row: DailyRow, onClick: () -> Unit, modifier: Modifier) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    val playing = row.state == "playing"
    val shape = SquircleShape(Radii.inner)
    val face = if (playing) {
        Modifier.background(tokens.action.copy(alpha = 0.10f), shape).border(1.5.dp, tokens.action, shape)
    } else {
        Modifier.background(tokens.canvas.copy(alpha = 0.35f), shape).border(1.dp, tokens.highlight, shape)
    }
    Box(modifier.height(148.dp).pressable(onClick = onClick).then(face)) {
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

/** One length to play: a fresh puzzle, or the one left in play. */
@Composable
private fun PlayCard(length: Int, inPlay: HistoryItem?, onClick: () -> Unit, modifier: Modifier) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    Card(modifier.height(104.dp).pressable(onClick = onClick), padding = PaddingValues(horizontal = 14.dp, vertical = 12.dp)) {
        Column(Modifier.fillMaxWidth()) {
            Row(horizontalArrangement = Arrangement.spacedBy(3.dp)) {
                repeat(length) {
                    Box(Modifier.size(10.dp).border(1.5.dp, tokens.tileEmpty, SquircleShape(Radii.miniTile)))
                }
            }
            Spacer(Modifier.weight(1f))
            Row {
                SurmiseText("$length", type.title, tokens.ink, Modifier.alignByBaseline())
                SurmiseText(stringResource(R.string.letters_suffix), type.body, tokens.inkDim, Modifier.alignByBaseline().padding(start = 4.dp))
            }
            if (inPlay != null) {
                SurmiseText(pluralStringResource(R.plurals.guesses_in, inPlay.attempts, inPlay.attempts), type.footnote, tokens.accentText, maxLines = 1)
            } else {
                SurmiseText(stringResource(R.string.new_puzzle), type.footnote, tokens.inkMuted, maxLines = 1)
            }
        }
    }
}

/** Home's state from today's date and the model's reads. */
fun homeState(name: String, daily: DailyView?, history: List<HistoryItem>, playedToday: Int, note: Throwable?): HomeState =
    HomeState(
        date = homeDate(),
        name = name,
        daily = daily,
        inPlay = (4..6).mapNotNull { n ->
            history.firstOrNull { it.kind == "random" && it.length == n && it.status == "in_progress" }?.let { n to it }
        }.toMap(),
        playedToday = playedToday,
        note = note,
    )
