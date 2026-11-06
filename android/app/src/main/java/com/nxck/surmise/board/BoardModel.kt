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

/** The engine calls a board's keys go to: a plain board's, or a sprint run's. */
interface Moves {
    suspend fun type(c: Char): BoardView
    suspend fun delete(): BoardView
    suspend fun trim(index: Int): BoardView
    suspend fun submit(): BoardView
    suspend fun view(): BoardView
}

/** A board on its own: the board.* methods. */
class BoardMoves(private val client: CoreClient) : Moves {
    override suspend fun type(c: Char) = client.boardType(TypeRequest(c.toString()))
    override suspend fun delete() = client.boardDelete()
    override suspend fun trim(index: Int) = client.boardTrim(TrimRequest(index))
    override suspend fun submit() = client.boardSubmit()
    override suspend fun view() = client.boardView()
}

/**
 * One open board. Every key is a call into the engine, which owns the typed
 * row as well as the guesses, so the screen never holds a copy that could
 * disagree with what gets saved.
 */
open class BoardModel(
    protected val client: CoreClient,
    protected val scope: CoroutineScope,
    initial: BoardView,
    /**
     * Leaves the board, for next's kind ("daily" goes to the daily screen), or
     * with null, back to where the board was opened from.
     */
    private val onLeave: (next: String?) -> Unit,
    /** Swaps this board for a fresh one. */
    private val onReplace: (BoardView) -> Unit,
    private val moves: Moves = BoardMoves(client),
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
        protected set

    var result by mutableStateOf<ResultView?>(null)
        private set
    var profile by mutableStateOf<ProfileView?>(null)
        private set

    /** Whether the result sheet is up. */
    var sheet by mutableStateOf(false)

    val done: Boolean get() = view.status == "won" || view.status == "lost"

    protected fun show(v: BoardView) {
        view = v
        viewAt = SystemClock.uptimeMillis()
    }

    protected fun refuse(e: Throwable) {
        note = e
        notes++
        if (isRowRefusal(e)) shakes++
    }

    fun clearNote() {
        note = null
    }

    fun type(c: Char) {
        if (done) return
        scope.launch { runCatching { moves.type(c) }.onSuccess(::show) }
    }

    fun delete() {
        scope.launch { runCatching { moves.delete() }.onSuccess(::show) }
    }

    /** Erases the typed row back to [index]: a tap on a typed tile. */
    fun trim(index: Int) {
        scope.launch { runCatching { moves.trim(index) }.onSuccess(::show) }
    }

    fun submit() {
        if (done) return
        scope.launch {
            val before = view.rows.size
            runCatching { moves.submit() }
                .onSuccess { v ->
                    note = null
                    show(v)
                    if (v.rows.size > before) revealing = v.rows.size - 1
                    if (done) finished()
                }
                .onFailure { e ->
                    // A guess the store failed to keep is still played: read
                    // the board again so it shows.
                    if ((e as? CoreException)?.code == "save_failed") {
                        runCatching { moves.view() }.onSuccess(::show)
                    }
                    refuse(e)
                }
        }
    }

    /** The screen's word that the turning row has landed. */
    fun revealed() {
        val wasLast = revealing >= 0 && done
        revealing = -1
        if (wasLast) landed()
    }

    /** The finishing row has turned over. */
    protected open fun landed() {
        if (result != null) sheet = true
    }

    /** A guess finished the board. */
    protected open suspend fun finished() = loadResult()

    private suspend fun loadResult(raise: Boolean = true) {
        runCatching { client.resultView() }.onSuccess { result = it }
        runCatching { client.recordProfile() }.onSuccess { profile = it }
        // With no row turning (motion off lands it at once) the sheet need
        // not wait on anything.
        if (raise && revealing < 0) sheet = true
    }

    /**
     * A board opened already over (from the history, say) loads its result
     * so "see result" can show it, but keeps the sheet down: the player came
     * to look at the grid.
     */
    fun loadOpened() {
        if (!done || result != null) return
        scope.launch { loadResult(raise = false) }
    }

    fun openResult() {
        if (result != null) sheet = true
    }

    /** Next from the result: a fresh board for a random one, the kind's own screen for the rest. */
    fun next() {
        val r = result
        if (r == null || r.next != "random") {
            onLeave(r?.next)
            return
        }
        scope.launch {
            runCatching { client.boardNew(NewBoardRequest(view.length)) }
                .onSuccess(onReplace)
                .onFailure(::refuse)
        }
    }

    /** Banks the running clock and saves. Safe to call twice. */
    open suspend fun leave() {
        runCatching { client.boardLeave() }.onSuccess(::show)
    }
}
