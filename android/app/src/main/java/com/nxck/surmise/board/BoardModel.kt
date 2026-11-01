package com.nxck.surmise.board

import android.os.SystemClock
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import com.nxck.surmise.core.gen.BoardView
import com.nxck.surmise.core.gen.CoreClient
import com.nxck.surmise.core.gen.CoreException
import com.nxck.surmise.core.gen.NewBoardRequest
import com.nxck.surmise.core.gen.ProfileView
import com.nxck.surmise.core.gen.ResultView
import com.nxck.surmise.core.gen.TrimRequest
import com.nxck.surmise.core.gen.TypeRequest
import com.nxck.surmise.isRowRefusal
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch

/**
 * One open board. Every key is a call into the engine, which owns the typed
 * row as well as the guesses, so the screen never holds a copy that could
 * disagree with what gets saved.
 */
class BoardModel(
    private val client: CoreClient,
    private val scope: CoroutineScope,
    initial: BoardView,
    /** Leaves for home: "home" on the result, or next for a kind that has no fresh board. */
    private val onHome: () -> Unit,
    /** Swaps this board for a fresh one. */
    private val onReplace: (BoardView) -> Unit,
) {
    var view by mutableStateOf(initial)
        private set

    /** When [view] arrived, on the uptime clock, so the shown time can run on. */
    var viewAt by mutableLongStateOf(SystemClock.uptimeMillis())
        private set

    /** The last refusal, and a count that ticks with each, so a repeat shows again. */
    var note by mutableStateOf<Throwable?>(null)
        private set
    var notes by mutableIntStateOf(0)
        private set

    /** Ticks on each refused row, which shakes it. */
    var shakes by mutableIntStateOf(0)
        private set

    /**
     * The row whose marks are turning over, or -1. The screen holds the
     * keyboard's marks back until it lands, so a key never gives a tile away.
     */
    var revealing by mutableIntStateOf(-1)
        private set

    var result by mutableStateOf<ResultView?>(null)
        private set
    var profile by mutableStateOf<ProfileView?>(null)
        private set

    /** Whether the result sheet is up. */
    var sheet by mutableStateOf(false)

    val done: Boolean get() = view.status == "won" || view.status == "lost"

    private fun show(v: BoardView) {
        view = v
        viewAt = SystemClock.uptimeMillis()
    }

    private fun refuse(e: Throwable) {
        note = e
        notes++
        if (isRowRefusal(e)) shakes++
    }

    fun clearNote() {
        note = null
    }

    fun type(c: Char) {
        if (done) return
        scope.launch { runCatching { client.boardType(TypeRequest(c.toString())) }.onSuccess(::show) }
    }

    fun delete() {
        scope.launch { runCatching { client.boardDelete() }.onSuccess(::show) }
    }

    /** Erases the typed row back to [index]: a tap on a typed tile. */
    fun trim(index: Int) {
        scope.launch { runCatching { client.boardTrim(TrimRequest(index)) }.onSuccess(::show) }
    }

    fun submit() {
        if (done) return
        scope.launch {
            val before = view.rows.size
            runCatching { client.boardSubmit() }
                .onSuccess { v ->
                    note = null
                    show(v)
                    if (v.rows.size > before) revealing = v.rows.size - 1
                    if (done) loadResult()
                }
                .onFailure { e ->
                    // A guess the store failed to keep is still played: read
                    // the board again so it shows.
                    if ((e as? CoreException)?.code == "save_failed") {
                        runCatching { client.boardView() }.onSuccess(::show)
                    }
                    refuse(e)
                }
        }
    }

    /** The screen's word that the turning row has landed. */
    fun revealed() {
        val wasLast = revealing >= 0 && done
        revealing = -1
        if (wasLast && result != null) sheet = true
    }

    private suspend fun loadResult() {
        runCatching { client.resultView() }.onSuccess { result = it }
        runCatching { client.recordProfile() }.onSuccess { profile = it }
        // With no row turning (motion off lands it at once) the sheet need
        // not wait on anything.
        if (revealing < 0) sheet = true
    }

    fun openResult() {
        if (result != null) sheet = true
    }

    /** Next from the result: a fresh board for a random one, home for the rest. */
    fun next() {
        val r = result
        if (r == null || r.next != "random") {
            onHome()
            return
        }
        scope.launch {
            runCatching { client.boardNew(NewBoardRequest(view.length)) }
                .onSuccess(onReplace)
                .onFailure(::refuse)
        }
    }

    /** Banks the running clock and saves. Safe to call twice. */
    suspend fun leave() {
        runCatching { client.boardLeave() }.onSuccess(::show)
    }
}
