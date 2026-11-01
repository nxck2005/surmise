package com.nxck.surmise

import com.nxck.surmise.core.gen.BestView
import com.nxck.surmise.core.gen.BoardView
import com.nxck.surmise.core.gen.CalendarView
import com.nxck.surmise.core.gen.CoreClient
import com.nxck.surmise.core.gen.DailyRow
import com.nxck.surmise.core.gen.DailyView
import com.nxck.surmise.core.gen.DayView
import com.nxck.surmise.core.gen.FormView
import com.nxck.surmise.core.gen.HistoryItem
import com.nxck.surmise.core.gen.ModeView
import com.nxck.surmise.core.gen.OpenerView
import com.nxck.surmise.core.gen.ProfileView
import com.nxck.surmise.core.gen.ResultView
import com.nxck.surmise.core.gen.Row
import com.nxck.surmise.core.gen.TrendsView
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
    winRate = 0.878,
    avgAttempts = 3.9,
    avgTimeMs = 94_000,
    currentStreak = 6,
    maxStreak = 11,
    distribution = listOf(1, 5, 12, 10, 6, 2),
    modes = listOf(
        ModeView(length = 4, played = 9, won = 8, winRate = 0.889, avgAttempts = 3.4, avgTimeMs = 61_000),
        ModeView(length = 5, played = 26, won = 23, winRate = 0.885, avgAttempts = 3.9, avgTimeMs = 98_000),
        ModeView(length = 6, played = 6, won = 5, winRate = 0.833, avgAttempts = 4.6, avgTimeMs = 132_000),
    ),
    playtimeMs = 45_600_000,
)

/** A year of play: a few games most days, more at weekends, a gap in august. */
val calendarFixture: CalendarView = run {
    val today = java.time.LocalDate.of(2026, 11, 1)
    val days = (0L until 300L).mapNotNull { back ->
        val d = today.minusDays(back)
        val n = ((back * 7 + back / 3) % 6).toInt() - 1 + if (d.dayOfWeek.value >= 6) 2 else 0
        if (n <= 0 || d.monthValue == 8) null else DayView(day = d.toString(), games = n, won = n - (back % 2).toInt().coerceAtMost(n))
    }.reversed()
    CalendarView(today = today.toString(), days = days)
}

val trendsFixture = mapOf(
    5 to TrendsView(
        length = 5,
        recentWindow = 20,
        recent = FormView(games = 20, won = 19, winRate = 0.95, avgAttempts = 3.6, avgTimeMs = 81_000),
        allTime = FormView(games = 26, won = 23, winRate = 0.885, avgAttempts = 3.9, avgTimeMs = 98_000),
        fastest = BestView(id = "r-5-3", attempts = 3, elapsedMs = 34_000),
        fewest = BestView(id = "r-5-8", attempts = 2, elapsedMs = 71_000),
        openers = listOf(
            OpenerView("crane", 12, 11, 0.917, 3.5),
            OpenerView("slate", 8, 7, 0.875, 3.9),
            OpenerView("audio", 4, 3, 0.75, 4.3),
            OpenerView("tears", 2, 2, 1.0, 3.0),
        ),
    ),
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

val historyItems = listOf(
    HistoryItem(id = "r-5-1", code = "042317", kind = "random", status = "in_progress", length = 5, attempts = 2, updatedAt = "2026-11-01T10:00:00Z"),
    HistoryItem(id = "d-4", code = "117204", kind = "daily", status = "won", length = 4, attempts = 3, updatedAt = "2026-11-01T09:00:00Z", daily = "2026-11-01"),
    HistoryItem(id = "c-6", code = "880913", kind = "challenge", status = "lost", length = 6, attempts = 7, updatedAt = "2026-10-30T19:30:00Z"),
    HistoryItem(id = "r-4-9", code = "300118", kind = "random", status = "won", length = 4, attempts = 4, updatedAt = "2026-10-29T08:12:00Z"),
)
