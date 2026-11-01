package com.nxck.surmise.sprint

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
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
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.nxck.surmise.Page
import com.nxck.surmise.R
import com.nxck.surmise.Segmented
import com.nxck.surmise.clockText
import com.nxck.surmise.core.gen.SprintOptions
import com.nxck.surmise.core.gen.SprintView
import com.nxck.surmise.design.CapsuleButton
import com.nxck.surmise.design.CapsuleStyle
import com.nxck.surmise.design.Card
import com.nxck.surmise.design.Sheet
import com.nxck.surmise.design.SurmiseText
import com.nxck.surmise.design.SurmiseTheme
import com.nxck.surmise.record.oneDecimal
import kotlinx.coroutines.delay

/** The sprint's setup: a length and a clock, then go. */
@Composable
fun SprintSetup(options: SprintOptions, startLength: Int, onStart: (length: Int, durationMs: Long) -> Unit, onBack: () -> Unit) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    var length by rememberSaveable { mutableIntStateOf(startLength) }
    var duration by rememberSaveable { mutableLongStateOf(options.defaultMs) }
    Page(stringResource(R.string.sprint), onBack) {
        SurmiseText(stringResource(R.string.sprint_about), type.body, tokens.inkDim, Modifier.padding(start = 20.dp, end = 20.dp, top = 4.dp))
        Card(Modifier.fillMaxWidth().padding(start = 16.dp, end = 16.dp, top = 20.dp)) {
            Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                SurmiseText(stringResource(R.string.sprint_length), type.body, tokens.ink)
                Segmented(listOf(4, 5, 6), length, { length = it }) { stringResource(R.string.letters, it) }
                if (options.durationsMs.isNotEmpty()) {
                    SurmiseText(stringResource(R.string.sprint_clock), type.body, tokens.ink, Modifier.padding(top = 8.dp))
                    Segmented(options.durationsMs, duration, { duration = it }) { stringResource(R.string.minutes, it / 60_000) }
                }
            }
        }
        CapsuleButton(
            stringResource(R.string.sprint_go),
            { onStart(length, duration) },
            Modifier.fillMaxWidth().padding(start = 16.dp, end = 16.dp, top = 20.dp),
        )
    }
}

/** The run's countdown for the top bar. At zero it asks the engine, which ends the run. */
@Composable
fun SprintClock(model: SprintModel) {
    val run = model.run
    var now by remember { mutableLongStateOf(android.os.SystemClock.uptimeMillis()) }
    val left = if (run.over) 0L else (run.leftMs - (now - model.runAt)).coerceAtLeast(0)
    LaunchedEffect(model.runAt, run.over) {
        while (!run.over) {
            now = android.os.SystemClock.uptimeMillis()
            val l = run.leftMs - (now - model.runAt)
            if (l <= 0) {
                model.tick()
                break
            }
            delay(l % 1000 + 1)
        }
    }
    // Round up, so 0:00 shows only once the run is over.
    val shown = (left + 999) / 1000 * 1000
    val tokens = SurmiseTheme.tokens
    SurmiseText(clockText(shown), SurmiseTheme.type.clock, if (left < 10_000) tokens.action else tokens.inkDim, maxLines = 1)
}

/** The run's tally for under the title. */
@Composable
fun sprintTally(run: SprintView): String = pluralStringResource(R.plurals.sprint_solved, run.solved, run.solved)

/** The summary once the clock runs out or the player stops. */
@Composable
fun SprintSummary(model: SprintModel, onAgain: () -> Unit, onHome: () -> Unit) {
    val run = model.run
    Sheet(visible = model.summary, onDismiss = onHome) {
        SprintSummaryBody(run, onAgain, onHome)
    }
}

@Composable
fun SprintSummaryBody(run: SprintView, onAgain: () -> Unit, onHome: () -> Unit) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    SurmiseText(pluralStringResource(R.plurals.sprint_solved, run.solved, run.solved), type.displaySmall, tokens.ink, Modifier.fillMaxWidth(), align = TextAlign.Center)
    SurmiseText(
        stringResource(R.string.sprint_in, clockText(run.durationMs), run.length),
        type.body,
        tokens.inkDim,
        Modifier.fillMaxWidth().padding(top = 4.dp),
        align = TextAlign.Center,
    )
    Row(Modifier.fillMaxWidth().padding(top = 20.dp)) {
        Figure("${run.dealt}", stringResource(R.string.sprint_dealt), Modifier.weight(1f))
        Figure("${run.accuracy}%", stringResource(R.string.sprint_accuracy), Modifier.weight(1f))
        Figure(if (run.solved > 0) oneDecimal(run.avgGuesses) else "–", stringResource(R.string.stat_avg_guesses), Modifier.weight(1f))
        Figure(if (run.solved > 0) clockText(run.solveTimeMs / run.solved) else "–", stringResource(R.string.sprint_per_word), Modifier.weight(1f))
    }
    Row(Modifier.fillMaxWidth().padding(top = 24.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        CapsuleButton(stringResource(R.string.home), onHome, Modifier.weight(1f), style = CapsuleStyle.Secondary)
        CapsuleButton(stringResource(R.string.sprint_again), onAgain, Modifier.weight(1f))
    }
}

@Composable
private fun Figure(value: String, label: String, modifier: Modifier) {
    Column(modifier, horizontalAlignment = Alignment.CenterHorizontally) {
        SurmiseText(value, SurmiseTheme.type.title, SurmiseTheme.tokens.ink)
        SurmiseText(label, SurmiseTheme.type.footnote, SurmiseTheme.tokens.inkMuted, maxLines = 1)
    }
}
