package com.nxck.surmise.sprint

import android.os.SystemClock
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import com.nxck.surmise.board.BoardModel
import com.nxck.surmise.board.Moves
import com.nxck.surmise.core.gen.BoardView
import com.nxck.surmise.core.gen.CoreClient
import com.nxck.surmise.core.gen.SprintView
import com.nxck.surmise.core.gen.TrimRequest
import com.nxck.surmise.core.gen.TypeRequest
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch

/** A run's keys go to sprint.*, and each answer carries the run as well as its board. */
private class SprintMoves(private val client: CoreClient, private val onRun: (SprintView) -> Unit) : Moves {
    private suspend fun board(v: SprintView): BoardView {
        onRun(v)
        return v.board
    }

    override suspend fun type(c: Char) = board(client.sprintType(TypeRequest(c.toString())))
    override suspend fun delete() = board(client.sprintDelete())
    override suspend fun trim(index: Int) = board(client.sprintTrim(TrimRequest(index)))
    override suspend fun submit() = board(client.sprintSubmit())
    override suspend fun view() = board(client.sprintView())
}

/**
 * A sprint: boards dealt one after another against a clock. A finished
 * board is followed by the next once its row has turned; there is no result
 * sheet until the clock runs out or the player stops, and then it is the
 * run's summary.
 */
class SprintModel private constructor(
    client: CoreClient,
    scope: CoroutineScope,
    initial: SprintView,
    onLeave: (String?) -> Unit,
    holder: Holder,
) : BoardModel(client, scope, initial.board, onLeave, {}, SprintMoves(client) { holder.run(it) }) {
    /** Lets the moves reach this model before its constructor has run. */
    private class Holder {
        var run: (SprintView) -> Unit = {}
    }

    constructor(client: CoreClient, scope: CoroutineScope, initial: SprintView, onLeave: (String?) -> Unit) :
        this(client, scope, initial, onLeave, Holder())

    var run by mutableStateOf(initial)
        private set

    /** When [run] arrived, on the uptime clock, so the countdown can run on. */
    var runAt by mutableLongStateOf(SystemClock.uptimeMillis())
        private set

    /** Whether the summary is up. */
    var summary by mutableStateOf(initial.over)
        private set

    init {
        holder.run = ::take
    }

    private fun take(v: SprintView) {
        run = v
        runAt = SystemClock.uptimeMillis()
        if (v.over) summary = true
    }

    override suspend fun finished() {
        // With nothing turning (motion off), deal at once.
        if (revealing < 0) deal()
    }

    override fun landed() {
        scope.launch { deal() }
    }

    private suspend fun deal() {
        if (run.over) return
        runCatching { client.sprintNext() }.onSuccess { v ->
            take(v)
            show(v.board)
        }
    }

    /** The countdown reached zero: ask the engine, which ends the run. */
    fun tick() {
        scope.launch { runCatching { client.sprintView() }.onSuccess { v -> take(v); show(v.board) } }
    }

    /** Stops the run and shows its summary. */
    fun end() {
        scope.launch { runCatching { client.sprintEnd() }.onSuccess(::take) }
    }

    /** Leaving a run ends it; the board the clock caught stays in the history. */
    override suspend fun leave() {
        if (!run.over) runCatching { client.sprintEnd() }.onSuccess { run = it }
    }
}
