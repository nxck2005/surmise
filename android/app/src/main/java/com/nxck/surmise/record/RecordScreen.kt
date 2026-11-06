package com.nxck.surmise.record

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.nxck.surmise.R
import com.nxck.surmise.clockText
import com.nxck.surmise.core.gen.CalendarView
import com.nxck.surmise.core.gen.ProfileView
import com.nxck.surmise.design.Card
import com.nxck.surmise.design.MeshGlow
import com.nxck.surmise.design.Radii
import com.nxck.surmise.design.SquircleShape
import com.nxck.surmise.design.SurmiseText
import com.nxck.surmise.design.SurmiseTheme
import com.nxck.surmise.design.TabBarHeight
import com.nxck.surmise.design.pressable
import kotlin.math.roundToInt

/**
 * The you tab's record: the streak with the four figures, the last half year
 * of play (a tap opens the year), each length, and the guesses wins took.
 */
@Composable
fun RecordScreen(profile: ProfileView?, calendar: CalendarView?, onYear: () -> Unit, modifier: Modifier = Modifier) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    Column(
        modifier
            .verticalScroll(rememberScrollState())
            .padding(bottom = TabBarHeight + 48.dp),
    ) {
        if (profile == null) return@Column
        if (profile.played == 0) {
            SurmiseText(stringResource(R.string.record_empty), type.body, tokens.inkMuted, Modifier.fillMaxWidth().padding(32.dp), align = TextAlign.Center)
            return@Column
        }
        StreakCard(profile, Modifier.padding(start = 16.dp, end = 16.dp, top = 14.dp))
        Card(
            Modifier.fillMaxWidth().padding(start = 16.dp, end = 16.dp, top = 10.dp).pressable(onClick = onYear),
            padding = PaddingValues(16.dp),
        ) {
            Column {
                Row(Modifier.fillMaxWidth().padding(bottom = 12.dp), verticalAlignment = Alignment.CenterVertically) {
                    SurmiseText(stringResource(R.string.record_year), type.section, tokens.ink, Modifier.weight(1f))
                    SurmiseText(stringResource(R.string.record_year_link), type.caption, tokens.accentText)
                }
                YearStrip(calendar, RECORD_WEEKS)
            }
        }
        if (profile.modes.isNotEmpty()) {
            SurmiseText(stringResource(R.string.record_by_length), type.section, tokens.ink, Modifier.padding(start = 20.dp, top = 22.dp, bottom = 10.dp))
            Card(Modifier.fillMaxWidth().padding(horizontal = 16.dp), padding = PaddingValues(horizontal = 16.dp, vertical = 6.dp)) {
                Column {
                    Row(Modifier.fillMaxWidth().height(32.dp), verticalAlignment = Alignment.CenterVertically) {
                        Box(Modifier.weight(1f))
                        Cell(stringResource(R.string.stat_played), head = true)
                        Cell(stringResource(R.string.stat_won), head = true)
                        Cell(stringResource(R.string.guesses), head = true)
                        Cell(stringResource(R.string.time), head = true)
                    }
                    for (m in profile.modes) {
                        Box(Modifier.fillMaxWidth().height(1.dp).background(tokens.ink.copy(alpha = 0.06f)))
                        Row(Modifier.fillMaxWidth().height(44.dp), verticalAlignment = Alignment.CenterVertically) {
                            SurmiseText(stringResource(R.string.letters, m.length), type.body, tokens.ink, Modifier.weight(1f))
                            Cell(m.played.toString())
                            Cell(percent(m.winRate))
                            Cell(oneDecimal(m.avgAttempts))
                            Cell(clockText(m.avgTimeMs))
                        }
                    }
                }
            }
        }
        if (profile.distribution.isNotEmpty()) {
            SurmiseText(stringResource(R.string.record_guesses), type.section, tokens.ink, Modifier.padding(start = 20.dp, top = 22.dp, bottom = 10.dp))
            Card(Modifier.fillMaxWidth().padding(horizontal = 16.dp)) {
                Bars(profile.distribution.mapIndexed { i, n -> "${i + 1}" to n })
            }
        }
    }
}

/** About half a year: what fits a phone's width at a readable cell. */
private const val RECORD_WEEKS = 25

@Composable
private fun androidx.compose.foundation.layout.RowScope.Figure(value: String, label: String) {
    Column(Modifier.weight(1f), horizontalAlignment = Alignment.CenterHorizontally) {
        SurmiseText(value, SurmiseTheme.type.title.copy(fontSize = SurmiseTheme.type.displaySmall.fontSize), SurmiseTheme.tokens.ink)
        SurmiseText(label, SurmiseTheme.type.footnote, SurmiseTheme.tokens.inkMuted)
    }
}

@Composable
private fun Cell(text: String, head: Boolean = false) {
    val type = SurmiseTheme.type
    val tokens = SurmiseTheme.tokens
    SurmiseText(text, if (head) type.footnote else type.caption, if (head) tokens.inkMuted else tokens.inkDim, Modifier.width(60.dp), align = TextAlign.End, maxLines = 1)
}

/** The win streak, with the four figures under it. */
@Composable
private fun StreakCard(p: ProfileView, modifier: Modifier) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    MeshGlow(modifier.fillMaxWidth().clip(SquircleShape(Radii.hero))) {
        Column(Modifier.padding(20.dp)) {
            Row(verticalAlignment = Alignment.Bottom) {
                SurmiseText("${p.currentStreak}", type.display, tokens.ink)
                SurmiseText(stringResource(R.string.record_streak), type.body, tokens.inkDim, Modifier.padding(start = 8.dp, bottom = 6.dp).weight(1f))
                SurmiseText(stringResource(R.string.record_best, p.maxStreak), type.caption, tokens.inkDim, Modifier.padding(bottom = 6.dp))
            }
            Row(Modifier.fillMaxWidth().padding(top = 16.dp)) {
                Figure(p.played.toString(), stringResource(R.string.stat_played))
                Figure(percent(p.winRate), stringResource(R.string.stat_won))
                Figure(oneDecimal(p.avgAttempts), stringResource(R.string.stat_avg_guesses))
                Figure(clockText(p.avgTimeMs), stringResource(R.string.stat_avg_time))
            }
        }
    }
}

/** Labelled bars, the largest in the chart's highlight. */
@Composable
fun Bars(rows: List<Pair<String, Int>>, modifier: Modifier = Modifier, labelWidth: androidx.compose.ui.unit.Dp = 18.dp) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    val top = rows.maxOfOrNull { it.second }?.coerceAtLeast(1) ?: 1
    Column(modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(6.dp)) {
        for ((label, n) in rows) {
            val on = n == top
            Row(Modifier.fillMaxWidth().height(16.dp), verticalAlignment = Alignment.CenterVertically) {
                SurmiseText(label, type.code, tokens.inkMuted, Modifier.width(labelWidth))
                Box(Modifier.weight(1f)) {
                    Box(
                        Modifier
                            .fillMaxWidth((n.toFloat() / top).coerceAtLeast(0.02f))
                            .height(16.dp)
                            .background(if (on) tokens.chartHighlight else tokens.chartNeutral, SquircleShape(4.dp)),
                    )
                }
                SurmiseText("$n", type.footnote, if (on) tokens.ink else tokens.inkMuted, Modifier.width(32.dp), align = TextAlign.End)
            }
        }
    }
}

fun percent(rate: Double): String = "${(rate * 100).roundToInt()}%"

fun oneDecimal(x: Double): String = if (x == 0.0) "0" else "%.1f".format(x)

/** "12h 40m", or "40m" under an hour. */
fun playtimeText(ms: Long): String {
    val m = ms / 60_000
    return if (m >= 60) "${m / 60}h ${m % 60}m" else "${m}m"
}
