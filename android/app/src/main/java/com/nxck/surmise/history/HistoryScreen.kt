package com.nxck.surmise.history

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.combinedClickable
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
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.nxck.surmise.R
import com.nxck.surmise.core.gen.HistoryItem
import com.nxck.surmise.design.CapsuleButton
import com.nxck.surmise.design.CapsuleShape
import com.nxck.surmise.design.CapsuleStyle
import com.nxck.surmise.design.Radii
import com.nxck.surmise.design.Sheet
import com.nxck.surmise.design.SquircleShape
import com.nxck.surmise.design.SurmiseText
import com.nxck.surmise.design.SurmiseTheme
import com.nxck.surmise.design.TabBarHeight
import com.nxck.surmise.design.pressable
import com.nxck.surmise.record.dayText
import java.time.LocalDate
import java.time.OffsetDateTime
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

private val Kinds = listOf("random", "daily", "challenge", "custom", "room", "sprint")
private val Statuses = listOf("in_progress", "won", "lost")

/**
 * Every saved puzzle, newest first. A tap opens it (to finish or to look
 * back over); a long press asks to delete it.
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
) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    var length by rememberSaveable { mutableStateOf(0) }
    var kind by rememberSaveable { mutableStateOf("") }
    var status by rememberSaveable { mutableStateOf("") }
    val filter = HistoryFilter(length, kind, status, day)
    val shown = items.filter(filter::matches)
    var asking by remember { mutableStateOf<HistoryItem?>(null) }

    Box(modifier.fillMaxSize()) {
        LazyColumn(
            Modifier.fillMaxSize().statusBarsPadding(),
            contentPadding = PaddingValues(bottom = TabBarHeight + 48.dp),
        ) {
            item {
                SurmiseText(stringResource(R.string.tab_history), type.display, tokens.ink, Modifier.padding(start = 20.dp, top = 18.dp))
            }
            item {
                Column(Modifier.padding(top = 14.dp, bottom = 14.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    if (day != "") DayChip(day, onClearDay)
                    Chips(listOf(0, 4, 5, 6), length, { length = it }) {
                        if (it == 0) stringResource(R.string.filter_any_length) else stringResource(R.string.letters, it)
                    }
                    Chips(listOf("") + Kinds, kind, { kind = it }) { kindLabel(it) }
                    Chips(listOf("") + Statuses, status, { status = it }) { statusLabel(it) }
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
            itemsIndexed(shown, key = { _, i -> i.id }) { index, item ->
                HistoryRow(
                    item,
                    first = index == 0,
                    last = index == shown.lastIndex,
                    onOpen = { onOpen(item) },
                    onLongPress = { asking = item },
                )
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
private fun androidx.compose.foundation.layout.RowScope.DangerButton(label: String, onClick: () -> Unit) {
    val tokens = SurmiseTheme.tokens
    Box(
        Modifier.weight(1f).height(40.dp).pressable(onClick = onClick).background(tokens.danger, CapsuleShape),
        contentAlignment = Alignment.Center,
    ) {
        SurmiseText(label, SurmiseTheme.type.body, tokens.onAction)
    }
}

@Composable
private fun <T> Chips(values: List<T>, selected: T, onSelect: (T) -> Unit, label: @Composable (T) -> String) {
    val tokens = SurmiseTheme.tokens
    LazyRow(contentPadding = PaddingValues(horizontal = 16.dp), horizontalArrangement = Arrangement.spacedBy(6.dp)) {
        items(values.size) { i ->
            val v = values[i]
            val on = v == selected
            Box(
                Modifier
                    .height(32.dp)
                    .pressable(onClick = { onSelect(v) })
                    .background(if (on) tokens.ink else tokens.card, CapsuleShape)
                    .padding(horizontal = 14.dp),
                contentAlignment = Alignment.Center,
            ) {
                SurmiseText(label(v), SurmiseTheme.type.caption, if (on) tokens.canvas else tokens.inkDim, maxLines = 1)
            }
        }
    }
}

/** The calendar's day, on: a tap clears it and shows every day again. */
@Composable
private fun DayChip(day: String, onClear: () -> Unit) {
    val tokens = SurmiseTheme.tokens
    val date = runCatching { LocalDate.parse(day) }.getOrNull()
    Box(
        Modifier
            .padding(horizontal = 16.dp)
            .height(32.dp)
            .pressable(onClick = onClear)
            .background(tokens.ink, CapsuleShape)
            .padding(horizontal = 14.dp),
        contentAlignment = Alignment.Center,
    ) {
        SurmiseText(stringResource(R.string.history_day, date?.let(::dayText) ?: day), SurmiseTheme.type.caption, tokens.canvas, maxLines = 1)
    }
}

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

@OptIn(ExperimentalFoundationApi::class)
@Composable
private fun HistoryRow(item: HistoryItem, first: Boolean, last: Boolean, onOpen: () -> Unit, onLongPress: () -> Unit) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    val haptic = LocalHapticFeedback.current
    val shape = SquircleShape(Radii.card, top = first, bottom = last)
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
                .padding(horizontal = 16.dp, vertical = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column(Modifier.weight(1f)) {
                SurmiseText(rowTitle(item), type.body, tokens.ink, maxLines = 1)
                SurmiseText(stringResource(R.string.code_prefix, item.code), type.code, tokens.inkMuted)
            }
            Column(horizontalAlignment = Alignment.End) {
                val (text, color) = when (item.status) {
                    "won" -> stringResource(R.string.history_solved, item.attempts, item.length + 1) to tokens.accentText
                    "lost" -> stringResource(R.string.history_failed) to tokens.inkDim
                    else -> stringResource(R.string.history_in_play, item.attempts, item.length + 1) to tokens.action
                }
                SurmiseText(text, type.caption, color)
                SurmiseText(shortDate(item.updatedAt), type.footnote, tokens.inkMuted)
            }
        }
        if (!last) Box(Modifier.fillMaxWidth().padding(start = 16.dp).height(1.dp).background(tokens.ink.copy(alpha = 0.06f)))
    }
}

@Composable
private fun rowTitle(i: HistoryItem): String = when (i.kind) {
    "daily" -> stringResource(R.string.board_daily, i.length)
    "custom" -> stringResource(R.string.board_custom, i.length)
    "challenge" -> stringResource(R.string.board_challenge, i.length)
    "room" -> stringResource(R.string.board_room, i.length)
    "sprint" -> stringResource(R.string.board_sprint, i.length)
    else -> stringResource(R.string.letters, i.length)
}

/** "nov 1" from the engine's RFC 3339 time, or "" when it does not parse. */
fun shortDate(rfc3339: String): String = runCatching {
    OffsetDateTime.parse(rfc3339).atZoneSameInstant(java.time.ZoneId.systemDefault())
        .format(DateTimeFormatter.ofPattern("MMM d", Locale.getDefault())).lowercase(Locale.getDefault())
}.getOrDefault("")
