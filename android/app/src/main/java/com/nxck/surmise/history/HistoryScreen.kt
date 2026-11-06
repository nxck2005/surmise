package com.nxck.surmise.history

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.rotate
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.nxck.surmise.MiniBoard
import com.nxck.surmise.R
import com.nxck.surmise.core.gen.HistoryItem
import com.nxck.surmise.design.CapsuleButton
import com.nxck.surmise.design.CapsuleShape
import com.nxck.surmise.design.CapsuleStyle
import com.nxck.surmise.design.Icon
import com.nxck.surmise.design.Radii
import com.nxck.surmise.design.Sheet
import com.nxck.surmise.design.SquircleShape
import com.nxck.surmise.design.SurmiseIcons
import com.nxck.surmise.design.SurmiseText
import com.nxck.surmise.design.SurmiseTheme
import com.nxck.surmise.design.TabBarHeight
import com.nxck.surmise.design.pressable
import com.nxck.surmise.record.dayText
import java.time.LocalDate
import java.time.OffsetDateTime
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.util.Locale

/**
 * The history's filter: 0 is any length, "" any kind, status or day. The
 * TUI's three fields, plus the calendar's day (YYYY-MM-DD, as the engine
 * files each puzzle).
 */
data class HistoryFilter(val length: Int = 0, val kind: String = "", val status: String = "", val day: String = "") {
    fun matches(i: HistoryItem): Boolean =
        (length == 0 || i.length == length) &&
            (kind == "" || i.kind == kind) &&
            (status == "" || i.status == status) &&
            (day == "" || i.day == day)
}

private val Lengths = listOf(0, 4, 5, 6)
private val Kinds = listOf("", "daily", "random", "challenge", "custom", "room", "sprint")
private val Statuses = listOf("", "in_progress", "won", "lost")

/** Which filter's list is open. */
private enum class Pick { Length, Kind, Status }

/**
 * The day a row is filed under: the engine's calendar day, or the local day
 * it was last played when it has none.
 */
fun historyDay(i: HistoryItem): LocalDate? =
    runCatching { LocalDate.parse(i.day) }.getOrNull()
        ?: runCatching { OffsetDateTime.parse(i.updatedAt).atZoneSameInstant(ZoneId.systemDefault()).toLocalDate() }.getOrNull()

/** The rows under their days, in the order the list has them (newest first). */
fun groupByDay(items: List<HistoryItem>): List<Pair<LocalDate?, List<HistoryItem>>> =
    items.groupBy(::historyDay).toList()

/** "wednesday, nov 4", with the year when it is not this one. */
fun dayHeading(d: LocalDate, today: LocalDate): String {
    val pattern = if (d.year == today.year) "EEEE, MMM d" else "EEEE, MMM d, yyyy"
    return d.format(DateTimeFormatter.ofPattern(pattern, Locale.getDefault())).lowercase(Locale.getDefault())
}

/**
 * Every saved puzzle, newest first, under its day. Three filters sit over
 * the list, each a chip that opens its choices. A tap opens a puzzle (to
 * finish or to look back over); a long press asks to delete it.
 */
@Composable
fun HistoryScreen(
    items: List<HistoryItem>,
    onOpen: (HistoryItem) -> Unit,
    onDelete: (HistoryItem) -> Unit,
    modifier: Modifier = Modifier,
    /** The calendar day picked on the year, or "" for every day. */
    day: String = "",
    onClearDay: () -> Unit = {},
    today: LocalDate = LocalDate.now(),
) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    var length by rememberSaveable { mutableStateOf(0) }
    var kind by rememberSaveable { mutableStateOf("") }
    var status by rememberSaveable { mutableStateOf("") }
    val filter = HistoryFilter(length, kind, status, day)
    val shown = items.filter(filter::matches)
    var asking by remember { mutableStateOf<HistoryItem?>(null) }
    var picking by remember { mutableStateOf<Pick?>(null) }

    Box(modifier.fillMaxSize()) {
        LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(bottom = TabBarHeight + 48.dp)) {
            item {
                Row(
                    Modifier.horizontalScroll(rememberScrollState()).padding(start = 16.dp, end = 16.dp, top = 14.dp, bottom = 4.dp),
                    horizontalArrangement = Arrangement.spacedBy(6.dp),
                ) {
                    FilterChip(lengthLabel(length), length != 0) { picking = Pick.Length }
                    FilterChip(kindLabel(kind), kind != "") { picking = Pick.Kind }
                    FilterChip(statusLabel(status), status != "") { picking = Pick.Status }
                    if (day != "") DayChip(day, onClearDay)
                }
            }
            if (shown.isEmpty()) {
                item {
                    SurmiseText(
                        stringResource(if (items.isEmpty()) R.string.history_empty else R.string.history_none_match),
                        type.body,
                        tokens.inkMuted,
                        Modifier.fillMaxWidth().padding(32.dp),
                        align = TextAlign.Center,
                    )
                }
            }
            for ((d, rows) in groupByDay(shown)) {
                item(key = "day:$d") {
                    val heading = when (d) {
                        null -> ""
                        today -> stringResource(R.string.history_today)
                        today.minusDays(1) -> stringResource(R.string.history_yesterday)
                        else -> dayHeading(d, today)
                    }
                    SurmiseText(heading, type.caption, tokens.inkMuted, Modifier.padding(start = 20.dp, top = 16.dp, bottom = 8.dp))
                }
                items(rows, key = { it.id }) { item ->
                    HistoryRow(
                        item,
                        first = item === rows.first(),
                        last = item === rows.last(),
                        onOpen = { onOpen(item) },
                        onLongPress = { asking = item },
                    )
                }
            }
        }
        val pick = picking
        Sheet(visible = pick != null, onDismiss = { picking = null }) {
            when (pick) {
                Pick.Length -> Choices(stringResource(R.string.history_pick_length), Lengths, length, { length = it; picking = null }) { lengthLabel(it) }
                Pick.Kind -> Choices(stringResource(R.string.history_pick_kind), Kinds, kind, { kind = it; picking = null }) { kindLabel(it) }
                Pick.Status -> Choices(stringResource(R.string.history_pick_status), Statuses, status, { status = it; picking = null }) { statusLabel(it) }
                null -> {}
            }
        }
        val pending = asking
        Sheet(visible = pending != null, onDismiss = { asking = null }) {
            if (pending != null) {
                SurmiseText(stringResource(R.string.delete_title, pending.code), type.displaySmall, tokens.ink)
                SurmiseText(
                    stringResource(if (pending.kind == "daily") R.string.delete_daily_body else R.string.delete_body),
                    type.body,
                    tokens.inkDim,
                    Modifier.padding(top = 6.dp),
                )
                Row(Modifier.fillMaxWidth().padding(top = 20.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    CapsuleButton(stringResource(R.string.keep), { asking = null }, Modifier.weight(1f), style = CapsuleStyle.Secondary)
                    DangerButton(stringResource(R.string.delete)) {
                        asking = null
                        onDelete(pending)
                    }
                }
            }
        }
    }
}

@Composable
internal fun androidx.compose.foundation.layout.RowScope.DangerButton(label: String, onClick: () -> Unit) {
    val tokens = SurmiseTheme.tokens
    Box(
        Modifier.weight(1f).height(40.dp).pressable(onClick = onClick).background(tokens.danger, CapsuleShape),
        contentAlignment = Alignment.Center,
    ) {
        SurmiseText(label, SurmiseTheme.type.body, tokens.onAction)
    }
}

/** A filter: its value and a chevron, tinted when it narrows the list. */
@Composable
private fun FilterChip(label: String, on: Boolean, onClick: () -> Unit) {
    val tokens = SurmiseTheme.tokens
    val ink = if (on) tokens.accentText else tokens.ink
    var m = Modifier
        .height(36.dp)
        .pressable(onClick = onClick)
        .background(if (on) tokens.accent.copy(alpha = 0.16f) else tokens.card, CapsuleShape)
    if (on) m = m.border(1.dp, tokens.accent.copy(alpha = 0.5f), CapsuleShape)
    Row(m.padding(horizontal = 14.dp), verticalAlignment = Alignment.CenterVertically) {
        SurmiseText(label, SurmiseTheme.type.caption, ink, maxLines = 1)
        Icon(SurmiseIcons.Chevron, ink.copy(alpha = 0.7f), Modifier.padding(start = 6.dp).rotate(90f), size = 10.dp)
    }
}

/** The calendar's day, on: a tap clears it and shows every day again. */
@Composable
private fun DayChip(day: String, onClear: () -> Unit) {
    val tokens = SurmiseTheme.tokens
    val date = runCatching { LocalDate.parse(day) }.getOrNull()
    Box(
        Modifier
            .height(36.dp)
            .pressable(onClick = onClear)
            .background(tokens.accent.copy(alpha = 0.16f), CapsuleShape)
            .border(1.dp, tokens.accent.copy(alpha = 0.5f), CapsuleShape)
            .padding(horizontal = 14.dp),
        contentAlignment = Alignment.Center,
    ) {
        SurmiseText(stringResource(R.string.history_day, date?.let(::dayText) ?: day), SurmiseTheme.type.caption, tokens.accentText, maxLines = 1)
    }
}

/** One filter's choices, the current one checked. */
@Composable
private fun <T> Choices(title: String, values: List<T>, selected: T, onPick: (T) -> Unit, label: @Composable (T) -> String) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    SurmiseText(title, type.displaySmall, tokens.ink, Modifier.padding(bottom = 8.dp))
    for (v in values) {
        Row(
            Modifier.fillMaxWidth().height(48.dp).pressable(onClick = { onPick(v) }),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            SurmiseText(label(v), type.body, if (v == selected) tokens.accentText else tokens.ink, Modifier.weight(1f))
            if (v == selected) Icon(SurmiseIcons.Check, tokens.accentText, size = 16.dp)
        }
    }
}

@Composable
private fun lengthLabel(n: Int): String =
    if (n == 0) stringResource(R.string.filter_any_length) else stringResource(R.string.letters, n)

@Composable
private fun kindLabel(kind: String): String = when (kind) {
    "random" -> stringResource(R.string.kind_random)
    "daily" -> stringResource(R.string.kind_daily)
    "challenge" -> stringResource(R.string.kind_challenge)
    "custom" -> stringResource(R.string.kind_custom)
    "room" -> stringResource(R.string.kind_room)
    "sprint" -> stringResource(R.string.kind_sprint)
    else -> stringResource(R.string.filter_any_kind)
}

@Composable
private fun statusLabel(status: String): String = when (status) {
    "in_progress" -> stringResource(R.string.status_playing)
    "won" -> stringResource(R.string.status_won)
    "lost" -> stringResource(R.string.status_lost)
    else -> stringResource(R.string.filter_any_status)
}

/** The engine's g/y/a marks as the mini board's mark names. */
fun miniMarks(marks: List<String>): List<List<String>> = marks.map { row ->
    row.map {
        when (it) {
            'g' -> "correct"
            'y' -> "present"
            else -> "absent"
        }
    }
}

@OptIn(ExperimentalFoundationApi::class)
@Composable
private fun HistoryRow(item: HistoryItem, first: Boolean, last: Boolean, onOpen: () -> Unit, onLongPress: () -> Unit) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    val haptic = LocalHapticFeedback.current
    val shape = SquircleShape(Radii.card, top = first, bottom = last)
    val tries = item.length + 1
    Column(Modifier.padding(horizontal = 16.dp).background(tokens.card, shape)) {
        Row(
            Modifier
                .fillMaxWidth()
                .combinedClickable(
                    interactionSource = remember { MutableInteractionSource() },
                    indication = null,
                    onClick = onOpen,
                    onLongClick = {
                        haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                        onLongPress()
                    },
                )
                .padding(horizontal = 14.dp, vertical = 10.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            // A fixed width keeps the titles in one column whatever the length.
            Box(Modifier.width(34.dp), contentAlignment = Alignment.Center) {
                MiniBoard(
                    item.length,
                    tries,
                    miniMarks(item.marks),
                    cell = if (item.length >= 6) 4.dp else 5.dp,
                    gap = 1.dp,
                )
            }
            Column(Modifier.weight(1f).padding(start = 12.dp)) {
                SurmiseText(rowTitle(item), type.body, tokens.ink, maxLines = 1)
                SurmiseText(shortDate(item.updatedAt), type.footnote, tokens.inkMuted)
            }
            val (text, color) = when (item.status) {
                "won" -> stringResource(R.string.history_solved, item.attempts, tries) to tokens.accentText
                "lost" -> stringResource(R.string.history_failed) to tokens.inkDim
                else -> stringResource(R.string.practice_resume, item.attempts, tries) to tokens.action
            }
            SurmiseText(text, type.caption, color, Modifier.padding(start = 8.dp), maxLines = 1)
        }
        if (!last) Box(Modifier.fillMaxWidth().padding(start = 60.dp).height(1.dp).background(tokens.ink.copy(alpha = 0.06f)))
    }
}

@Composable
private fun rowTitle(i: HistoryItem): String = when (i.kind) {
    "daily" -> stringResource(R.string.board_daily, i.length)
    "custom" -> stringResource(R.string.board_custom, i.length)
    "challenge" -> stringResource(R.string.board_challenge, i.length)
    "room" -> stringResource(R.string.board_room, i.length)
    "sprint" -> stringResource(R.string.board_sprint, i.length)
    else -> stringResource(R.string.board_practice, i.length)
}

/** "nov 1" from the engine's RFC 3339 time, or "" when it does not parse. */
fun shortDate(rfc3339: String): String = runCatching {
    OffsetDateTime.parse(rfc3339).atZoneSameInstant(ZoneId.systemDefault())
        .format(DateTimeFormatter.ofPattern("MMM d", Locale.getDefault())).lowercase(Locale.getDefault())
}.getOrDefault("")
