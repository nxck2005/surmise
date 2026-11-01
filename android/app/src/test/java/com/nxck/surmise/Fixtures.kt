package com.nxck.surmise

import com.nxck.surmise.core.gen.BoardView
import com.nxck.surmise.core.gen.CoreClient
import com.nxck.surmise.core.gen.DailyRow
import com.nxck.surmise.core.gen.DailyView
import com.nxck.surmise.core.gen.HistoryItem
import com.nxck.surmise.core.gen.ModeView
import com.nxck.surmise.core.gen.ProfileView
import com.nxck.surmise.core.gen.ResultView
import com.nxck.surmise.core.gen.Row
import com.nxck.surmise.core.gen.Trio
import com.nxck.surmise.core.gen.coreJson
import com.nxck.surmise.home.HomeState
import kotlinx.serialization.KSerializer

// Fixed engine answers for the screen tests, shaped as the Go side writes
// them, so a frame never depends on the word lists or today's date.

private fun marks(vararg rows: String): List<List<String>> = rows.map { r ->
    r.map { c ->
        when (c) {
            'c' -> "correct"
            'p' -> "present"
            else -> "absent"
        }
    }
}

private fun row(guess: String, m: String) = Row(guess, marks(m).single())

val boardInPlay = BoardView(
    id = "r-5-1",
    code = "042317",
    length = 5,
    maxAttempts = 6,
    rows = listOf(row("crane", "aacac"), row("toast", "capca")),
    typing = "tw",
    keys = mapOf("c" to "absent", "r" to "absent", "a" to "present", "n" to "absent", "e" to "correct", "t" to "correct", "o" to "present", "s" to "absent"),
    status = "in_progress",
    elapsedMs = 102_000,
    kind = "random",
    canRestart = true,
)

val boardWon = boardInPlay.copy(
    rows = boardInPlay.rows + row("tweet", "ccccc"),
    typing = "",
    status = "won",
    answer = "tweet",
    keys = boardInPlay.keys + mapOf("w" to "correct"),
)

val resultWon = ResultView(board = boardWon, attempts = "3/6", share = "surmise #042317 3/6", fastest = true, next = "random")

val profile = ProfileView(
    displayName = "nick",
    played = 41,
    won = 36,
    currentStreak = 6,
    distribution = listOf(1, 5, 12, 10, 6, 2),
    modes = listOf(ModeView(length = 5, played = 41, won = 36, winRate = 0.87)),
)

val dailyToday = DailyView(
    day = "2026-11-01",
    rows = listOf(
        DailyRow(length = 4, state = "won", attempts = 3, maxAttempts = 5, marks = marks("apaa", "paca", "cccc")),
        DailyRow(length = 5, state = "playing", attempts = 2, maxAttempts = 6, marks = marks("aacac", "capca")),
        DailyRow(length = 6, state = "new", maxAttempts = 7),
    ),
    trio = Trio(done = 1, of = 3),
)

val homeFixture = HomeState(
    date = "sunday, november 1",
    name = "nick",
    daily = dailyToday,
    inPlay = mapOf(5 to HistoryItem(id = "r-5-1", kind = "random", status = "in_progress", length = 5, attempts = 2)),
    playedToday = 1284,
)

/** A client that answers each method from [answers], an envelope per method. */
fun fakeClient(answers: Map<String, String>): CoreClient = CoreClient { method, _ ->
    answers[method] ?: """{"err":{"code":"unknown_method","detail":"$method"}}"""
}

fun <T> ok(serializer: KSerializer<T>, value: T): String = """{"ok":${coreJson.encodeToString(serializer, value)}}"""

fun refusal(code: String): String = """{"err":{"code":"$code","detail":""}}"""
