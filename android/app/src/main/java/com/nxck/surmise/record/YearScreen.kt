package com.nxck.surmise.record

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import com.nxck.surmise.BackButton
import com.nxck.surmise.R
import com.nxck.surmise.Segmented
import com.nxck.surmise.clockText
import com.nxck.surmise.core.gen.CalendarView
import com.nxck.surmise.core.gen.DayView
import com.nxck.surmise.core.gen.FormView
import com.nxck.surmise.core.gen.TrendsView
import com.nxck.surmise.design.Card
import com.nxck.surmise.design.SquircleShape
import com.nxck.surmise.design.SurmiseText
import com.nxck.surmise.design.SurmiseTheme
import com.nxck.surmise.design.pressable
import java.time.DayOfWeek
import java.time.LocalDate
import java.time.format.DateTimeFormatter
import java.util.Locale

/** How many weeks the year grid shows, ending with this one. */
const val YEAR_WEEKS = 53

/**
 * The year: a day grid of play, then each length's form (the last games
 * against all time), its bests, and the openers used most.
 */
@Composable
fun YearScreen(
    calendar: CalendarView?,
    trends: Map<Int, TrendsView>,
    onBack: () -> Unit,
    startLength: Int = 5,
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
        Row(Modifier.fillMaxWidth().height(44.dp).padding(horizontal = 16.dp).padding(top = 8.dp), verticalAlignment = Alignment.CenterVertically) {
            BackButton(onBack)
        }
        SurmiseText(stringResource(R.string.year_title), type.display, tokens.ink, Modifier.padding(start = 20.dp, top = 20.dp))
        if (calendar != null) YearCard(calendar, Modifier.padding(start = 16.dp, end = 16.dp, top = 18.dp))

        var length by rememberSaveable { mutableIntStateOf(startLength) }
        SurmiseText(stringResource(R.string.trends_title), type.section, tokens.ink, Modifier.padding(start = 20.dp, top = 24.dp, bottom = 10.dp))
        Segmented(listOf(4, 5, 6), length, { length = it }, Modifier.padding(horizontal = 16.dp)) { stringResource(R.string.letters, it) }
        val t = trends[length]
        if (t == null || t.allTime.games == 0) {
            SurmiseText(stringResource(R.string.trends_none, length), type.body, tokens.inkMuted, Modifier.padding(start = 20.dp, top = 16.dp))
            return@Column
        }
        FormCard(t, Modifier.padding(start = 16.dp, end = 16.dp, top = 10.dp))
        if (t.fastest != null || t.fewest != null) {
            Row(Modifier.padding(start = 16.dp, end = 16.dp, top = 10.dp), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                t.fastest?.let { Best(stringResource(R.string.trends_fastest), clockText(it.elapsedMs), pluralStringResource(R.plurals.guesses_count, it.attempts, it.attempts), Modifier.weight(1f)) }
                t.fewest?.let { Best(stringResource(R.string.trends_fewest), "${it.attempts}", clockText(it.elapsedMs), Modifier.weight(1f)) }
            }
        }
        if (t.openers.isNotEmpty()) {
            SurmiseText(stringResource(R.string.trends_openers), type.section, tokens.ink, Modifier.padding(start = 20.dp, top = 24.dp, bottom = 10.dp))
            Card(Modifier.fillMaxWidth().padding(horizontal = 16.dp)) {
                Bars(t.openers.take(5).map { it.word to it.games }, labelWidth = 64.dp)
            }
        }
    }
}

/** The grid of weeks, a column each, with the tapped day's line under it. */
@Composable
private fun YearCard(c: CalendarView, modifier: Modifier) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    val today = runCatching { LocalDate.parse(c.today) }.getOrNull() ?: return
    val byDay = c.days.associateBy { it.day }
    var picked by rememberSaveable { mutableStateOf(c.today) }
    // The grid starts on the monday YEAR_WEEKS-1 weeks before this week's.
    val start = today.with(DayOfWeek.MONDAY).minusWeeks((YEAR_WEEKS - 1).toLong())
    Card(modifier.fillMaxWidth()) {
        Column {
            BoxWithConstraints(Modifier.fillMaxWidth()) {
                val gap = 2.dp
                val cell = ((maxWidth - gap * (YEAR_WEEKS - 1)) / YEAR_WEEKS).coerceAtMost(14.dp)
                Column {
                    MonthLabels(start, cell + gap)
                    Row(Modifier.padding(top = 4.dp), horizontalArrangement = Arrangement.spacedBy(gap)) {
                        for (w in 0 until YEAR_WEEKS) {
                            Column(verticalArrangement = Arrangement.spacedBy(gap)) {
                                for (d in 0 until 7) {
                                    val day = start.plusDays(w * 7L + d)
                                    if (day.isAfter(today)) {
                                        Box(Modifier.size(cell))
                                    } else {
                                        val key = day.toString()
                                        val on = key == picked
                                        Box(
                                            Modifier
                                                .size(cell)
                                                .pressable(onClick = { picked = key })
                                                .background(if (on) tokens.ink else heat(byDay[key]?.games ?: 0), SquircleShape(1.dp)),
                                        )
                                    }
                                }
                            }
                        }
                    }
                }
            }
            Row(Modifier.fillMaxWidth().padding(top = 14.dp), verticalAlignment = Alignment.CenterVertically) {
                val d = byDay[picked]
                val date = runCatching { LocalDate.parse(picked) }.getOrNull()
                SurmiseText(date?.let(::dayText) ?: "", type.caption, tokens.ink, Modifier.weight(1f))
                SurmiseText(
                    if (d == null || d.games == 0) stringResource(R.string.year_day_none) else dayLine(d),
                    type.caption,
                    tokens.inkDim,
                )
            }
            Row(Modifier.fillMaxWidth().padding(top = 10.dp), horizontalArrangement = Arrangement.End, verticalAlignment = Alignment.CenterVertically) {
                SurmiseText(stringResource(R.string.year_less), type.footnote, tokens.inkMuted, Modifier.padding(end = 6.dp))
                for (n in listOf(0, 1, 2, 4, 6, 8)) {
                    Box(Modifier.padding(horizontal = 1.dp).size(10.dp).background(heat(n), SquircleShape(2.dp)))
                }
                SurmiseText(stringResource(R.string.year_more), type.footnote, tokens.inkMuted, Modifier.padding(start = 6.dp))
            }
        }
    }
}

/** Month initials over the first week that holds the 1st of each month. */
@Composable
private fun MonthLabels(start: LocalDate, step: Dp) {
    Box(Modifier.fillMaxWidth().height(14.dp)) {
        for (w in 0 until YEAR_WEEKS) {
            val weekStart = start.plusWeeks(w.toLong())
            val first = (0 until 7).map { weekStart.plusDays(it.toLong()) }.firstOrNull { it.dayOfMonth == 1 } ?: continue
            if (w > YEAR_WEEKS - 3) continue
            SurmiseText(
                monthText(first),
                SurmiseTheme.type.footnote,
                SurmiseTheme.tokens.inkMuted,
                Modifier.offset(x = step * w),
                maxLines = 1,
            )
        }
    }
}

/** The year grid's five steps of the chart colour, by games that day. */
@Composable
private fun heat(games: Int): Color {
    val c = SurmiseTheme.tokens.chartHighlight
    return when {
        games <= 0 -> SurmiseTheme.tokens.ink.copy(alpha = 0.06f)
        games == 1 -> c.copy(alpha = 0.3f)
        games == 2 -> c.copy(alpha = 0.5f)
        games <= 4 -> c.copy(alpha = 0.7f)
        games <= 7 -> c.copy(alpha = 0.85f)
        else -> c
    }
}

@Composable
private fun dayLine(d: DayView): String =
    pluralStringResource(R.plurals.year_played, d.games, d.games) + "  " + stringResource(R.string.year_won, d.won)

/** "oct". */
fun monthText(d: LocalDate): String =
    d.format(DateTimeFormatter.ofPattern("MMM", Locale.getDefault())).lowercase(Locale.getDefault())

/** "wed, october 28". */
fun dayText(d: LocalDate): String =
    d.format(DateTimeFormatter.ofPattern("EEE, MMMM d", Locale.getDefault())).lowercase(Locale.getDefault())

/** The last games against all time: won, guesses and time, side by side. */
@Composable
private fun FormCard(t: TrendsView, modifier: Modifier) {
    Card(modifier.fillMaxWidth(), padding = PaddingValues(horizontal = 16.dp, vertical = 8.dp)) {
        Column {
            Row(Modifier.fillMaxWidth().height(32.dp), verticalAlignment = Alignment.CenterVertically) {
                Box(Modifier.weight(1f))
                Head(stringResource(R.string.trends_recent, t.recent.games))
                Head(stringResource(R.string.trends_all_time))
            }
            FormRow(stringResource(R.string.stat_won), t.recent, t.allTime) { percent(it.winRate) }
            FormRow(stringResource(R.string.stat_avg_guesses), t.recent, t.allTime) { oneDecimal(it.avgAttempts) }
            FormRow(stringResource(R.string.stat_avg_time), t.recent, t.allTime) { clockText(it.avgTimeMs) }
        }
    }
}

@Composable
private fun Head(text: String) {
    SurmiseText(text, SurmiseTheme.type.footnote, SurmiseTheme.tokens.inkMuted, Modifier.width(84.dp), align = TextAlign.End)
}

@Composable
private fun FormRow(label: String, recent: FormView, all: FormView, value: (FormView) -> String) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    Box(Modifier.fillMaxWidth().height(1.dp).background(tokens.ink.copy(alpha = 0.06f)))
    Row(Modifier.fillMaxWidth().height(40.dp), verticalAlignment = Alignment.CenterVertically) {
        SurmiseText(label, type.body, tokens.ink, Modifier.weight(1f))
        SurmiseText(value(recent), type.body, tokens.ink, Modifier.width(84.dp), align = TextAlign.End)
        SurmiseText(value(all), type.body, tokens.inkDim, Modifier.width(84.dp), align = TextAlign.End)
    }
}

@Composable
private fun Best(label: String, value: String, note: String, modifier: Modifier) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    Card(modifier) {
        Column {
            SurmiseText(label, type.footnote, tokens.inkMuted)
            SurmiseText(value, type.displaySmall, tokens.ink, Modifier.padding(top = 4.dp))
            SurmiseText(note, type.caption, tokens.inkDim)
        }
    }
}
