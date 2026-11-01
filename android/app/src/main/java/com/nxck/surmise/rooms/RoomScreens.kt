package com.nxck.surmise.rooms

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.nxck.surmise.MiniBoard
import com.nxck.surmise.Page
import com.nxck.surmise.R
import com.nxck.surmise.Segmented
import com.nxck.surmise.clockText
import com.nxck.surmise.core.gen.Note
import com.nxck.surmise.core.gen.RankView
import com.nxck.surmise.core.gen.RoomEntryView
import com.nxck.surmise.core.gen.RoomPlayerView
import com.nxck.surmise.core.gen.RoomResultView
import com.nxck.surmise.core.gen.RoomView
import com.nxck.surmise.design.CapsuleButton
import com.nxck.surmise.design.CapsuleShape
import com.nxck.surmise.design.CapsuleStyle
import com.nxck.surmise.design.Card
import com.nxck.surmise.design.Radii
import com.nxck.surmise.design.SquircleShape
import com.nxck.surmise.design.Switch
import com.nxck.surmise.design.SurmiseText
import com.nxck.surmise.design.SurmiseTheme
import com.nxck.surmise.design.Tile
import com.nxck.surmise.design.TileState
import com.nxck.surmise.design.pressable
import com.nxck.surmise.noteText
import com.nxck.surmise.refusalText
import kotlinx.coroutines.delay
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle
import java.util.Locale

/** What the rooms screens show besides the room: the last refusal and note. */
data class RoomLines(val error: Throwable? = null, val note: Note? = null)

/** The rooms hub: make a room, join one by its code, or go back to one of ours. */
@Composable
fun RoomsScreen(
    list: List<RoomEntryView>,
    lines: RoomLines,
    startLength: Int,
    onMake: (mode: String, length: Int, autoStart: Boolean) -> Unit,
    onJoin: (String) -> Unit,
    onLeave: (String) -> Unit,
    onBack: () -> Unit,
) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    var mode by rememberSaveable { mutableStateOf("live") }
    var length by rememberSaveable { mutableIntStateOf(startLength) }
    var auto by rememberSaveable { mutableStateOf(true) }
    var code by rememberSaveable { mutableStateOf("") }
    Page(stringResource(R.string.rooms), onBack) {
        SurmiseText(stringResource(R.string.rooms_about), type.body, tokens.inkDim, Modifier.padding(start = 20.dp, end = 20.dp, top = 4.dp))

        Section(R.string.rooms_make)
        Card(Modifier.fillMaxWidth().padding(horizontal = 16.dp)) {
            Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
                Segmented(listOf("live", "async"), mode, { mode = it }) {
                    stringResource(if (it == "live") R.string.rooms_live else R.string.rooms_async)
                }
                Segmented(listOf(4, 5, 6), length, { length = it }) { stringResource(R.string.letters, it) }
                if (mode == "live") {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        SurmiseText(stringResource(R.string.rooms_auto_start), type.body, tokens.ink, Modifier.weight(1f))
                        Switch(auto, { auto = it })
                    }
                }
                CapsuleButton(stringResource(R.string.rooms_make_button), { onMake(mode, length, auto) }, Modifier.fillMaxWidth())
            }
        }

        Section(R.string.rooms_join)
        Card(Modifier.fillMaxWidth().padding(horizontal = 16.dp)) {
            Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
                CodeField(code, { code = it }, onGo = { if (code.isNotBlank()) onJoin(code) })
                CapsuleButton(stringResource(R.string.rooms_join_button), { onJoin(code) }, Modifier.fillMaxWidth(), enabled = code.isNotBlank())
            }
        }
        Lines(lines)

        if (list.isNotEmpty()) {
            Section(R.string.rooms_yours)
            Card(Modifier.fillMaxWidth().padding(horizontal = 16.dp), padding = PaddingValues(horizontal = 16.dp, vertical = 4.dp)) {
                Column {
                    list.forEachIndexed { i, e ->
                        if (i > 0) Box(Modifier.fillMaxWidth().height(1.dp).background(tokens.ink.copy(alpha = 0.06f)))
                        EntryRow(e, onOpen = { onJoin(e.code) }, onLeave = { onLeave(e.code) })
                    }
                }
            }
        }
    }
}

@Composable
private fun Section(title: Int) {
    SurmiseText(stringResource(title), SurmiseTheme.type.section, SurmiseTheme.tokens.ink, Modifier.padding(start = 20.dp, top = 24.dp, bottom = 10.dp))
}

@Composable
private fun CodeField(code: String, onChange: (String) -> Unit, onGo: () -> Unit) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    Box(
        Modifier.fillMaxWidth().height(48.dp).background(tokens.canvas.copy(alpha = 0.5f), SquircleShape(12.dp)).padding(horizontal = 14.dp),
        contentAlignment = Alignment.CenterStart,
    ) {
        if (code.isEmpty()) SurmiseText(stringResource(R.string.rooms_join_hint), type.code, tokens.inkMuted)
        BasicTextField(
            value = code,
            onValueChange = { if (it.length <= 8) onChange(it.uppercase()) },
            singleLine = true,
            textStyle = type.code.copy(color = tokens.ink),
            cursorBrush = SolidColor(tokens.cursor),
            keyboardOptions = KeyboardOptions(
                capitalization = KeyboardCapitalization.Characters,
                autoCorrectEnabled = false,
                keyboardType = KeyboardType.Ascii,
                imeAction = ImeAction.Go,
            ),
            keyboardActions = KeyboardActions(onGo = { onGo() }),
            modifier = Modifier.fillMaxWidth(),
        )
    }
}

@Composable
private fun EntryRow(e: RoomEntryView, onOpen: () -> Unit, onLeave: () -> Unit) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    Row(Modifier.fillMaxWidth().height(56.dp).pressable(onClick = onOpen), verticalAlignment = Alignment.CenterVertically) {
        Column(Modifier.weight(1f)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                SurmiseText(e.code, type.code, tokens.ink)
                if (e.open) SurmiseText(stringResource(R.string.rooms_open), type.footnote, tokens.accentText, Modifier.padding(start = 8.dp))
            }
            val day = dayText(e.joinedAtMs)
            SurmiseText(
                stringResource(if (e.mode == "async") R.string.rooms_mode_async else R.string.rooms_mode_live, day),
                type.footnote,
                tokens.inkMuted,
            )
        }
        SurmiseText(stringResource(R.string.rooms_leave), type.caption, tokens.danger, Modifier.pressable(onClick = onLeave).padding(8.dp))
    }
}

/** The last refusal and the last note, as lines under the cards. */
@Composable
private fun Lines(lines: RoomLines) {
    val context = LocalContext.current
    val tokens = SurmiseTheme.tokens
    val text = lines.error?.let { refusalText(context, it) } ?: lines.note?.let { noteText(context, it.code) } ?: return
    val good = lines.error == null && lines.note?.code in setOf("updated", "round_running")
    SurmiseText(text, SurmiseTheme.type.body, if (good) tokens.inkDim else tokens.danger, Modifier.padding(start = 20.dp, end = 20.dp, top = 12.dp))
}

/** What a room's actions do, from the lobby to the results. */
data class RoomActions(
    val onReady: (Boolean) -> Unit = {},
    val onStart: () -> Unit = {},
    val onRematch: () -> Unit = {},
    val onRefresh: () -> Unit = {},
    val onShare: (String) -> Unit = {},
    val onLeave: () -> Unit = {},
    /** Steps away and keeps the seat: back, and done on the results. */
    val onAway: () -> Unit = {},
)

/** The open room, as its screen says: connecting, the lobby, watching or the results. */
@Composable
fun RoomScreen(room: RoomView, lines: RoomLines, actions: RoomActions, now: () -> Long = System::currentTimeMillis) {
    val title = if (room.code.isEmpty()) stringResource(R.string.rooms) else stringResource(R.string.room_title, room.code)
    Page(title, actions.onAway) {
        when (room.screen) {
            "lobby" -> Lobby(room, actions, now)
            "watch" -> Watch(room, now)
            "result" -> room.results?.let { Results(room, it, actions) }
            else -> Connecting(room)
        }
        if (room.reconnecting && room.screen != "") {
            SurmiseText(stringResource(R.string.room_reconnecting), SurmiseTheme.type.caption, SurmiseTheme.tokens.inkMuted, Modifier.padding(start = 20.dp, top = 12.dp))
        }
        Lines(lines)
        if (room.active) {
            SurmiseText(
                stringResource(R.string.room_leave),
                SurmiseTheme.type.body,
                SurmiseTheme.tokens.danger,
                Modifier.fillMaxWidth().padding(top = 28.dp).pressable(onClick = actions.onLeave).padding(12.dp),
                align = TextAlign.Center,
            )
        }
    }
}

@Composable
private fun Connecting(room: RoomView) {
    val text = if (room.reconnecting) R.string.room_reconnecting else R.string.room_connecting
    SurmiseText(stringResource(text), SurmiseTheme.type.body, SurmiseTheme.tokens.inkDim, Modifier.padding(start = 20.dp, top = 8.dp))
}

@Composable
private fun Lobby(room: RoomView, actions: RoomActions, now: () -> Long) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    val async = room.mode == "async"
    SurmiseText(
        stringResource(if (async) R.string.room_line_async else R.string.room_line_live, room.length),
        type.caption,
        tokens.inkMuted,
        Modifier.padding(start = 20.dp),
    )
    // The code is the thing to hand on: big, and one tap from the share sheet.
    val shareText = stringResource(R.string.room_share, room.code)
    Card(Modifier.fillMaxWidth().padding(start = 16.dp, end = 16.dp, top = 18.dp)) {
        Column(Modifier.fillMaxWidth(), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(14.dp)) {
            Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                for (c in room.code) Tile(c, TileState.Typed, size = 40.dp)
            }
            CapsuleButton(stringResource(R.string.share), { actions.onShare(shareText) }, style = CapsuleStyle.Secondary)
        }
    }

    Section(R.string.room_players)
    Card(Modifier.fillMaxWidth().padding(horizontal = 16.dp), padding = PaddingValues(horizontal = 16.dp, vertical = 4.dp)) {
        Column {
            room.players.forEachIndexed { i, p ->
                if (i > 0) Box(Modifier.fillMaxWidth().height(1.dp).background(tokens.ink.copy(alpha = 0.06f)))
                PlayerRow(p, host = p.seat == room.host, async = async, length = room.length)
            }
        }
    }

    val me = room.players.firstOrNull { it.you }
    Column(Modifier.fillMaxWidth().padding(start = 16.dp, end = 16.dp, top = 20.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
        when {
            room.phase == "countdown" -> Countdown(room.goAtMs, now)
            async -> {
                CapsuleButton(stringResource(R.string.room_start_own), actions.onStart, Modifier.fillMaxWidth())
                if (room.expiresAtMs > 0) {
                    SurmiseText(stringResource(R.string.room_open_until, timeText(room.expiresAtMs)), type.footnote, tokens.inkMuted, Modifier.fillMaxWidth(), align = TextAlign.Center)
                }
            }
            else -> {
                val ready = me?.ready == true
                if (room.isHost && !room.autoStart) {
                    CapsuleButton(stringResource(R.string.room_start), actions.onStart, Modifier.fillMaxWidth())
                } else {
                    CapsuleButton(
                        stringResource(if (ready) R.string.room_not_yet else R.string.room_im_ready),
                        { actions.onReady(!ready) },
                        Modifier.fillMaxWidth(),
                        style = if (ready) CapsuleStyle.Secondary else CapsuleStyle.Primary,
                    )
                    val wait = if (room.autoStart) R.string.room_waiting_all else R.string.room_waiting_host
                    SurmiseText(stringResource(wait), type.footnote, tokens.inkMuted, Modifier.fillMaxWidth(), align = TextAlign.Center)
                }
            }
        }
    }
}

/** "starting in 3", counting down to the round. */
@Composable
private fun Countdown(goAt: Long, now: () -> Long) {
    val left = rememberLeft(goAt, now)
    Card(Modifier.fillMaxWidth()) {
        Column(Modifier.fillMaxWidth(), horizontalAlignment = Alignment.CenterHorizontally) {
            SurmiseText(stringResource(R.string.room_starting), SurmiseTheme.type.caption, SurmiseTheme.tokens.inkDim)
            SurmiseText("${(left + 999) / 1000}", SurmiseTheme.type.display, SurmiseTheme.tokens.ink)
        }
    }
}

/** Milliseconds left to [at], ticking each second. */
@Composable
private fun rememberLeft(at: Long, now: () -> Long): Long {
    var t by remember { mutableLongStateOf(now()) }
    LaunchedEffect(at) {
        while (at > t) {
            delay((at - t) % 1000 + 1)
            t = now()
        }
    }
    return (at - t).coerceAtLeast(0)
}

/** The time left in a live round, for the board's top bar. */
@Composable
fun RoundClock(endsAt: Long, now: () -> Long = System::currentTimeMillis) {
    if (endsAt <= 0) return
    SurmiseText(clockText(rememberLeft(endsAt, now)), SurmiseTheme.type.clock, SurmiseTheme.tokens.inkDim, maxLines = 1)
}

@Composable
private fun PlayerRow(p: RoomPlayerView, host: Boolean, async: Boolean, length: Int) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    val (state, on) = when {
        !p.connected && !async -> stringResource(R.string.room_away) to false
        async && p.done -> doneText(p.solved, p.guesses, length) to p.solved
        async -> "" to false
        p.ready -> stringResource(R.string.room_ready) to true
        else -> stringResource(R.string.room_not_ready) to false
    }
    Row(Modifier.fillMaxWidth().height(48.dp), verticalAlignment = Alignment.CenterVertically) {
        Box(Modifier.size(8.dp).background(if (on) tokens.tiles.correct.fill else tokens.ink.copy(alpha = 0.16f), CapsuleShape))
        SurmiseText(nameOf(p.name, p.seat), type.body, tokens.ink, Modifier.padding(start = 12.dp), maxLines = 1)
        val tags = listOfNotNull(
            stringResource(R.string.room_you).takeIf { p.you },
            stringResource(R.string.room_host).takeIf { host && !async },
        )
        if (tags.isNotEmpty()) SurmiseText(tags.joinToString(", "), type.footnote, tokens.inkMuted, Modifier.padding(start = 8.dp))
        Box(Modifier.weight(1f))
        SurmiseText(state, type.caption, if (on) tokens.ink else tokens.inkMuted, maxLines = 1)
    }
}

@Composable
private fun nameOf(name: String, seat: Int): String = name.ifBlank { stringResource(R.string.room_player, seat) }

@Composable
private fun doneText(solved: Boolean, guesses: Int, length: Int): String =
    if (solved) "$guesses/${length + 1}" else stringResource(R.string.room_failed)

/** A live round we sit out: everyone's marks as they come, and the time left. */
@Composable
private fun Watch(room: RoomView, now: () -> Long) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    Row(Modifier.fillMaxWidth().padding(horizontal = 20.dp), verticalAlignment = Alignment.CenterVertically) {
        SurmiseText(stringResource(R.string.room_watching), type.body, tokens.inkDim, Modifier.weight(1f))
        RoundClock(room.endsAtMs, now)
    }
    Boards(room.players.filter { it.inRound }.map { Triple(nameOf(it.name, it.seat), it.rows, it.solved) }, room.length)
}

/** Each player's board as marks, two to a row. */
@Composable
private fun Boards(boards: List<Triple<String, List<List<String>>, Boolean>>, length: Int) {
    val tokens = SurmiseTheme.tokens
    Column(Modifier.padding(start = 16.dp, end = 16.dp, top = 16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
        for (pair in boards.chunked(2)) {
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                for ((name, rows, solved) in pair) {
                    Card(Modifier.weight(1f), padding = PaddingValues(14.dp)) {
                        Column(Modifier.fillMaxWidth(), horizontalAlignment = Alignment.CenterHorizontally) {
                            MiniBoard(length, length + 1, rows, cell = 14.dp, gap = 3.dp)
                            SurmiseText(name, SurmiseTheme.type.caption, if (solved) tokens.ink else tokens.inkDim, Modifier.padding(top = 10.dp), maxLines = 1)
                        }
                    }
                }
                if (pair.size == 1) Box(Modifier.weight(1f))
            }
        }
    }
}

@Composable
private fun Results(room: RoomView, r: RoomResultView, actions: RoomActions) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    SurmiseText(stringResource(R.string.room_round, r.round), type.caption, tokens.inkMuted, Modifier.padding(start = 20.dp))
    // The word is the moment: its tiles, all green.
    Card(Modifier.fillMaxWidth().padding(start = 16.dp, end = 16.dp, top = 18.dp), radius = Radii.hero) {
        Column(Modifier.fillMaxWidth(), horizontalAlignment = Alignment.CenterHorizontally) {
            SurmiseText(stringResource(R.string.room_the_word), type.caption, tokens.inkDim)
            Row(Modifier.padding(top = 12.dp), horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                for (c in r.word) Tile(c, TileState.Correct, size = 44.dp)
            }
        }
    }
    Card(Modifier.fillMaxWidth().padding(start = 16.dp, end = 16.dp, top = 12.dp), padding = PaddingValues(horizontal = 16.dp, vertical = 4.dp)) {
        Column {
            r.ranking.forEachIndexed { i, k ->
                if (i > 0) Box(Modifier.fillMaxWidth().height(1.dp).background(tokens.ink.copy(alpha = 0.06f)))
                RankRow(i + 1, k, r.length)
            }
        }
    }
    Column(Modifier.fillMaxWidth().padding(start = 16.dp, end = 16.dp, top = 20.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
        if (room.mode == "live" && r.canRematch) {
            CapsuleButton(stringResource(R.string.room_rematch), actions.onRematch, Modifier.fillMaxWidth())
        }
        if (r.canRefresh) {
            CapsuleButton(stringResource(R.string.room_refresh), actions.onRefresh, Modifier.fillMaxWidth(), style = CapsuleStyle.Secondary)
        }
        CapsuleButton(stringResource(R.string.room_step_away), actions.onAway, Modifier.fillMaxWidth(), style = CapsuleStyle.Secondary)
    }
}

@Composable
private fun RankRow(place: Int, k: RankView, length: Int) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    Row(Modifier.fillMaxWidth().padding(vertical = 10.dp), verticalAlignment = Alignment.CenterVertically) {
        SurmiseText("$place", type.section, if (place == 1 && k.solved) tokens.accentText else tokens.inkMuted, Modifier.width(28.dp))
        MiniBoard(length, length + 1, k.rows, cell = 6.dp, gap = 1.5.dp)
        Column(Modifier.weight(1f).padding(start = 14.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                SurmiseText(nameOf(k.name, k.seat), type.body, tokens.ink, maxLines = 1)
                if (k.you) SurmiseText(stringResource(R.string.room_you), type.footnote, tokens.inkMuted, Modifier.padding(start = 8.dp))
            }
            SurmiseText(doneText(k.solved, k.guesses, length), type.caption, if (k.solved) tokens.inkDim else tokens.inkMuted)
        }
        if (k.solved) SurmiseText(clockText(k.elapsedMs), type.code, tokens.inkDim)
    }
}

// Formats sit outside the composables, which may not read the locale as state.

private fun timeText(ms: Long): String =
    DateTimeFormatter.ofLocalizedTime(FormatStyle.SHORT).withLocale(Locale.getDefault())
        .format(Instant.ofEpochMilli(ms).atZone(ZoneId.systemDefault())).lowercase(Locale.getDefault())

private fun dayText(ms: Long): String =
    DateTimeFormatter.ofPattern("MMM d", Locale.getDefault())
        .format(Instant.ofEpochMilli(ms).atZone(ZoneId.systemDefault())).lowercase(Locale.getDefault())
