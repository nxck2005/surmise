package com.nxck.surmise.friends

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
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
import com.nxck.surmise.R
import com.nxck.surmise.Segmented
import com.nxck.surmise.core.gen.RoomEntryView
import com.nxck.surmise.design.CapsuleButton
import com.nxck.surmise.design.CapsuleStyle
import com.nxck.surmise.design.Card
import com.nxck.surmise.design.Icon
import com.nxck.surmise.design.Sheet
import com.nxck.surmise.design.SquircleShape
import com.nxck.surmise.design.SurmiseIcons
import com.nxck.surmise.design.SurmiseText
import com.nxck.surmise.design.SurmiseTheme
import com.nxck.surmise.design.Switch
import com.nxck.surmise.design.TabBarHeight
import com.nxck.surmise.design.pressable
import com.nxck.surmise.modes.ChallengeState
import com.nxck.surmise.refusalText
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.util.Locale

/** What a typed code is, by its length once spaces and dashes are out. */
enum class CodeKind { Room, Challenge }

/** A code as typed, without spaces or dashes, in capitals. */
fun normalizeCode(raw: String): String = raw.filter { it != ' ' && it != '-' }.uppercase(Locale.ROOT)

/** A room's code has 6 characters and a challenge's 16; anything else is neither yet. */
fun codeKind(code: String): CodeKind? = when (code.length) {
    6 -> CodeKind.Room
    16 -> CodeKind.Challenge
    else -> null
}

/** The sheet up over the friends tab, if any. */
enum class FriendsSheet { Room, Challenge }

/** Everything the friends tab shows. */
data class FriendsState(
    /** Whether rooms can be played: online, and the server has them. */
    val rooms: Boolean,
    val entries: List<RoomEntryView> = emptyList(),
    /** The last refusal for a typed code. */
    val note: Throwable? = null,
    /** A room code typed while rooms are off. */
    val offline: Boolean = false,
)

/** What the friends tab's taps do. */
data class FriendsActions(
    val onCode: (String) -> Unit = {},
    val onSheet: (FriendsSheet) -> Unit = {},
    val onCustom: () -> Unit = {},
    /** Opens settings, where online play is turned on. */
    val onOnline: () -> Unit = {},
    val onRoom: (String) -> Unit = {},
)

/** The friends tab: one field for any code, the ways to start something, and our rooms. */
@Composable
fun FriendsScreen(state: FriendsState, actions: FriendsActions, modifier: Modifier = Modifier) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    val context = LocalContext.current
    Column(
        modifier
            .verticalScroll(rememberScrollState())
            .statusBarsPadding()
            .padding(bottom = TabBarHeight + 48.dp),
    ) {
        SurmiseText(stringResource(R.string.tab_friends), type.display, tokens.ink, Modifier.padding(start = 20.dp, top = 18.dp))
        SurmiseText(stringResource(R.string.friends_line), type.body, tokens.inkDim, Modifier.padding(start = 20.dp, end = 20.dp, top = 4.dp))

        CodeCard(actions.onCode, Modifier.padding(start = 16.dp, end = 16.dp, top = 18.dp))
        val note = when {
            state.offline -> stringResource(R.string.friends_rooms_offline)
            state.note != null -> refusalText(context, state.note)
            else -> null
        }
        if (note != null) SurmiseText(note, type.body, tokens.danger, Modifier.padding(start = 20.dp, end = 20.dp, top = 10.dp))

        Section(R.string.friends_start)
        Card(Modifier.fillMaxWidth().padding(horizontal = 16.dp), padding = PaddingValues(0.dp)) {
            Column {
                if (state.rooms) {
                    WayRow("live", R.string.rooms_make, stringResource(R.string.friends_room_line), first = true) { actions.onSheet(FriendsSheet.Room) }
                } else {
                    WayRow("live", R.string.rooms_make, stringResource(R.string.friends_needs_online), first = true, muted = true, onClick = actions.onOnline)
                }
                WayRow("code", R.string.friends_challenge, stringResource(R.string.friends_challenge_line)) { actions.onSheet(FriendsSheet.Challenge) }
                WayRow("abc", R.string.friends_custom, stringResource(R.string.friends_custom_line), onClick = actions.onCustom)
            }
        }

        if (state.entries.isNotEmpty()) {
            Section(R.string.rooms_yours)
            Card(Modifier.fillMaxWidth().padding(horizontal = 16.dp), padding = PaddingValues(0.dp)) {
                Column {
                    state.entries.forEachIndexed { i, e -> EntryRow(e, first = i == 0) { actions.onRoom(e.code) } }
                }
            }
        }
    }
}

@Composable
private fun Section(title: Int) {
    SurmiseText(stringResource(title), SurmiseTheme.type.section, SurmiseTheme.tokens.ink, Modifier.padding(start = 20.dp, top = 24.dp, bottom = 10.dp))
}

/** "got a code?": one field for a room's code or a challenge's. */
@Composable
private fun CodeCard(onOpen: (String) -> Unit, modifier: Modifier) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    var code by rememberSaveable { mutableStateOf("") }
    val ready = codeKind(normalizeCode(code)) != null
    Card(modifier.fillMaxWidth(), padding = PaddingValues(16.dp)) {
        Column {
            SurmiseText(stringResource(R.string.friends_code_label), type.caption, tokens.inkDim)
            Row(Modifier.fillMaxWidth().padding(top = 10.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Box(
                    Modifier.weight(1f).height(44.dp).background(tokens.canvas.copy(alpha = 0.5f), SquircleShape(12.dp)).padding(horizontal = 14.dp),
                    contentAlignment = Alignment.CenterStart,
                ) {
                    if (code.isEmpty()) SurmiseText(stringResource(R.string.friends_code_hint), type.body, tokens.inkMuted, maxLines = 1)
                    BasicTextField(
                        value = code,
                        onValueChange = { if (it.length <= 24) code = it.uppercase(Locale.ROOT) },
                        singleLine = true,
                        textStyle = type.code.copy(color = tokens.ink),
                        cursorBrush = SolidColor(tokens.cursor),
                        keyboardOptions = KeyboardOptions(
                            capitalization = KeyboardCapitalization.Characters,
                            autoCorrectEnabled = false,
                            keyboardType = KeyboardType.Ascii,
                            imeAction = ImeAction.Go,
                        ),
                        keyboardActions = KeyboardActions(onGo = { if (ready) onOpen(code) }),
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
                CapsuleButton(stringResource(R.string.friends_open), { onOpen(code) }, enabled = ready)
            }
        }
    }
}

/** One way to start something: a small mark, its name and what it does. */
@Composable
private fun WayRow(mark: String, title: Int, line: String, first: Boolean = false, muted: Boolean = false, onClick: () -> Unit) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    Column {
        if (!first) Box(Modifier.fillMaxWidth().height(1.dp).background(tokens.highlight))
        Row(
            Modifier.fillMaxWidth().height(64.dp).pressable(onClick = onClick).padding(horizontal = 16.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Box(
                Modifier.size(40.dp).background(tokens.canvas.copy(alpha = 0.5f), SquircleShape(12.dp)),
                contentAlignment = Alignment.Center,
            ) {
                SurmiseText(mark, type.code, if (muted) tokens.inkMuted else tokens.accentText, maxLines = 1)
            }
            Column(Modifier.weight(1f).padding(start = 14.dp)) {
                SurmiseText(stringResource(title), type.body.copy(fontWeight = type.section.fontWeight), if (muted) tokens.inkDim else tokens.ink, maxLines = 1)
                SurmiseText(line, type.footnote, if (muted) tokens.accentText else tokens.inkDim, maxLines = 1)
            }
            Icon(SurmiseIcons.Chevron, tokens.inkMuted, size = 12.dp)
        }
    }
}

/** One of our rooms: its code and state, its mode and when we joined. It opens the room; leaving is in there. */
@Composable
private fun EntryRow(e: RoomEntryView, first: Boolean, onOpen: () -> Unit) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    Column {
        if (!first) Box(Modifier.fillMaxWidth().height(1.dp).background(tokens.highlight))
        Row(
            Modifier.fillMaxWidth().height(60.dp).pressable(onClick = onOpen).padding(horizontal = 16.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column(Modifier.weight(1f)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    SurmiseText(e.code, type.code, tokens.ink)
                    if (e.open) SurmiseText(stringResource(R.string.rooms_open), type.footnote, tokens.accentText, Modifier.padding(start = 8.dp))
                }
                SurmiseText(
                    stringResource(if (e.mode == "async") R.string.rooms_mode_async else R.string.rooms_mode_live, dayText(e.joinedAtMs)),
                    type.footnote,
                    tokens.inkMuted,
                )
            }
            Icon(SurmiseIcons.Chevron, tokens.inkMuted, size = 12.dp)
        }
    }
}

private fun dayText(ms: Long): String =
    DateTimeFormatter.ofPattern("MMM d", Locale.getDefault())
        .format(Instant.ofEpochMilli(ms).atZone(ZoneId.systemDefault())).lowercase(Locale.getDefault())

/** The friends tab's sheets: make a room, or send a challenge. */
@Composable
fun FriendsSheets(
    sheet: FriendsSheet?,
    startLength: Int,
    roomError: Throwable?,
    challenge: ChallengeState,
    onDismiss: () -> Unit,
    onMakeRoom: (mode: String, length: Int, autoStart: Boolean) -> Unit,
    onMakeChallenge: (Int) -> Unit,
    onShare: (String) -> Unit,
    onPlayChallenge: (String) -> Unit,
) {
    Sheet(visible = sheet != null, onDismiss = onDismiss) {
        when (sheet) {
            FriendsSheet.Room -> MakeRoom(startLength, roomError, onMakeRoom)
            FriendsSheet.Challenge -> SendChallenge(challenge, startLength, onMakeChallenge, onShare, onPlayChallenge)
            null -> Unit
        }
    }
}

@Composable
private fun MakeRoom(startLength: Int, error: Throwable?, onMake: (String, Int, Boolean) -> Unit) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    val context = LocalContext.current
    var mode by rememberSaveable { mutableStateOf("live") }
    var length by rememberSaveable { mutableIntStateOf(startLength) }
    var auto by rememberSaveable { mutableStateOf(true) }
    SurmiseText(stringResource(R.string.rooms_make), type.displaySmall, tokens.ink)
    Row(Modifier.fillMaxWidth().padding(top = 16.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        ModeOption(R.string.rooms_live, R.string.friends_live_line, mode == "live", Modifier.weight(1f)) { mode = "live" }
        ModeOption(R.string.rooms_async, R.string.friends_async_line, mode == "async", Modifier.weight(1f)) { mode = "async" }
    }
    SurmiseText(stringResource(R.string.friends_length), type.caption, tokens.inkDim, Modifier.padding(top = 16.dp, bottom = 8.dp))
    Segmented(listOf(4, 5, 6), length, { length = it }) { stringResource(R.string.letters, it) }
    if (mode == "live") {
        Row(Modifier.fillMaxWidth().padding(top = 16.dp), verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) {
                SurmiseText(stringResource(R.string.rooms_auto_start), type.body, tokens.ink)
                SurmiseText(stringResource(R.string.friends_auto_note), type.footnote, tokens.inkMuted)
            }
            Switch(auto, { auto = it })
        }
    }
    CapsuleButton(stringResource(R.string.rooms_make_button), { onMake(mode, length, mode == "live" && auto) }, Modifier.fillMaxWidth().padding(top = 20.dp))
    SurmiseText(stringResource(R.string.friends_room_after), type.footnote, tokens.inkMuted, Modifier.fillMaxWidth().padding(top = 10.dp), align = TextAlign.Center)
    if (error != null) SurmiseText(refusalText(context, error), type.body, tokens.danger, Modifier.padding(top = 10.dp))
}

@Composable
private fun ModeOption(title: Int, line: Int, on: Boolean, modifier: Modifier, onClick: () -> Unit) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    val shape = SquircleShape(16.dp)
    Column(
        modifier
            .pressable(onClick = onClick)
            .background(if (on) tokens.accent.copy(alpha = 0.12f) else tokens.canvas.copy(alpha = 0.5f), shape)
            .border(if (on) 1.5.dp else 1.dp, if (on) tokens.accent else tokens.highlight, shape)
            .padding(12.dp),
    ) {
        SurmiseText(stringResource(title), type.body.copy(fontWeight = type.section.fontWeight), tokens.ink)
        SurmiseText(stringResource(line), type.footnote, tokens.inkDim, Modifier.padding(top = 2.dp))
    }
}

@Composable
private fun SendChallenge(state: ChallengeState, startLength: Int, onMake: (Int) -> Unit, onShare: (String) -> Unit, onPlay: (String) -> Unit) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    val context = LocalContext.current
    var length by rememberSaveable { mutableIntStateOf(startLength) }
    SurmiseText(stringResource(R.string.friends_challenge), type.displaySmall, tokens.ink)
    SurmiseText(stringResource(R.string.challenge_about), type.body, tokens.inkDim, Modifier.padding(top = 6.dp))
    Segmented(listOf(4, 5, 6), length, { length = it }, Modifier.padding(top = 16.dp)) { stringResource(R.string.letters, it) }
    if (state.made.isNotEmpty() && state.madeLength == length) {
        val shareText = stringResource(R.string.challenge_share, state.made)
        SurmiseText(state.made, type.title, tokens.ink, Modifier.fillMaxWidth().padding(top = 18.dp), align = TextAlign.Center)
        Row(Modifier.fillMaxWidth().padding(top = 16.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            CapsuleButton(stringResource(R.string.challenge_play_it), { onPlay(state.made) }, Modifier.weight(1f), style = CapsuleStyle.Secondary)
            CapsuleButton(stringResource(R.string.share), { onShare(shareText) }, Modifier.weight(1f))
        }
    } else {
        CapsuleButton(stringResource(R.string.challenge_make_code), { onMake(length) }, Modifier.fillMaxWidth().padding(top = 20.dp))
    }
    state.note?.let { SurmiseText(refusalText(context, it), type.body, tokens.danger, Modifier.padding(top = 10.dp)) }
}
