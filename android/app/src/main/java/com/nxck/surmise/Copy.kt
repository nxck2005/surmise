package com.nxck.surmise

import android.content.Context
import com.nxck.surmise.core.gen.BoardView
import com.nxck.surmise.core.gen.CoreException
import java.time.LocalDate
import java.time.format.DateTimeFormatter
import java.util.Locale

// The words the engine leaves to the front end. Go answers with codes; each
// code gets one plain line here, and nothing raw from Go reaches the player.

/** The line for a refusal. [length] fills "needs 5 letters". */
fun refusalText(context: Context, e: Throwable, length: Int = 0): String {
    val code = (e as? CoreException)?.code ?: "internal"
    return when (code) {
        "too_short" -> context.getString(R.string.err_too_short, length)
        "not_a_word" -> context.getString(R.string.err_not_a_word)
        "finished" -> context.getString(R.string.err_finished)
        "save_failed" -> context.getString(R.string.err_save_failed)
        "new_puzzle_failed" -> context.getString(R.string.err_new_puzzle_failed)
        "daily_spent" -> context.getString(R.string.err_daily_spent)
        "not_found" -> context.getString(R.string.err_not_found)
        "challenge_spent" -> context.getString(R.string.err_challenge_spent)
        "bad_challenge" -> context.getString(R.string.err_bad_challenge)
        "sprint_over" -> context.getString(R.string.err_sprint_over)
        "offline" -> context.getString(R.string.err_offline)
        "too_many_rooms" -> context.getString(R.string.err_too_many_rooms)
        "bad_room_code" -> context.getString(R.string.err_bad_room_code)
        "no_room" -> context.getString(R.string.err_no_room)
        "restart_daily", "restart_custom", "restart_challenge", "restart_room", "restart_sprint" ->
            context.getString(R.string.err_restart)
        else -> context.getString(R.string.err_other, code)
    }
}

/** The line for a room's note, or null for one that needs no line. */
fun noteText(context: Context, code: String): String? = when (code) {
    "no_room" -> context.getString(R.string.note_no_room)
    "full" -> context.getString(R.string.note_full)
    "started" -> context.getString(R.string.note_started)
    "bad_key" -> context.getString(R.string.note_bad_key)
    "left" -> context.getString(R.string.note_left)
    "unreachable" -> context.getString(R.string.note_unreachable)
    "rooms_off" -> context.getString(R.string.note_rooms_off)
    "save_failed" -> context.getString(R.string.note_save_failed)
    "round_running" -> context.getString(R.string.note_round_running)
    "room_ended" -> context.getString(R.string.note_room_ended)
    "not_playing" -> context.getString(R.string.note_not_playing)
    "reconnecting" -> context.getString(R.string.note_reconnecting)
    "room_closed" -> context.getString(R.string.note_room_closed)
    "refresh_failed" -> context.getString(R.string.note_refresh_failed)
    "updated" -> context.getString(R.string.note_updated)
    else -> null
}

/** A refusal the player can fix by typing: it shakes the row. */
fun isRowRefusal(e: Throwable): Boolean = (e as? CoreException)?.code in setOf("too_short", "not_a_word")

/** What a board is, for its top bar: "5 letters", "daily, 5 letters". */
fun boardTitle(context: Context, b: BoardView): String = when (b.kind) {
    "daily" -> context.getString(R.string.board_daily, b.length)
    "custom" -> context.getString(R.string.board_custom, b.length)
    "challenge" -> context.getString(R.string.board_challenge, b.length)
    "sprint" -> context.getString(R.string.board_sprint, b.length)
    "room" -> context.getString(R.string.board_room, b.length)
    else -> context.getString(R.string.letters, b.length)
}

/** A play time as the TUI writes it: 0:42, 12:05, 1:02:09. */
fun clockText(ms: Long): String {
    val s = (ms.coerceAtLeast(0) / 1000)
    val h = s / 3600
    val m = (s / 60) % 60
    val sec = s % 60
    return if (h > 0) "%d:%02d:%02d".format(h, m, sec) else "%d:%02d".format(m, sec)
}

/** Today's date as home's top line: "sunday, november 1". */
fun homeDate(day: LocalDate = LocalDate.now()): String =
    day.format(DateTimeFormatter.ofPattern("EEEE, MMMM d", Locale.getDefault())).lowercase(Locale.getDefault())

/** A count with grouping, as "1,284". */
fun countText(n: Int): String = "%,d".format(n)
