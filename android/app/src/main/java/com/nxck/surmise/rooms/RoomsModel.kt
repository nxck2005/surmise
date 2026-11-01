package com.nxck.surmise.rooms

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import com.nxck.surmise.core.gen.CoreClient
import com.nxck.surmise.core.gen.IDRequest
import com.nxck.surmise.core.gen.MakeRoomRequest
import com.nxck.surmise.core.gen.Note
import com.nxck.surmise.core.gen.ReadyRequest
import com.nxck.surmise.core.gen.RoomCode
import com.nxck.surmise.core.gen.RoomEntryView
import com.nxck.surmise.core.gen.RoomView
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch

/** A note the board shows as a refused row, not as a line. */
private val rowNotes = setOf("not_a_word", "too_short")

/**
 * The room session, as the engine holds it. No call waits on the network:
 * each answers with the view as it stands, and the server's answers come
 * later as "room" events, after which [refresh] reads the view again.
 *
 * [onGone] runs when the room we were in, or were opening, is no longer
 * there: it ended, we left, or the join failed (the note says which).
 */
class RoomsModel(
    private val client: CoreClient,
    private val scope: CoroutineScope,
    private val onGone: () -> Unit,
) {
    var room by mutableStateOf(RoomView())
        private set

    /** The rooms this install is in, newest first. */
    var list by mutableStateOf(emptyList<RoomEntryView>())
        private set

    /** The last call the engine refused: offline, too many rooms, a bad code. */
    var error by mutableStateOf<Throwable?>(null)
        private set

    /** The last note the room sent. Each has its own seq, so a repeat shows again. */
    var note by mutableStateOf<Note?>(null)
        private set

    /** Our board, while a round of ours is on. */
    var board by mutableStateOf<RoomBoardModel?>(null)
        private set

    /** Whether a make or a join is on its way and the room not yet open. */
    var opening by mutableStateOf(false)
        private set

    private var seen = -1

    fun refresh() {
        scope.launch { runCatching { client.roomsView() }.onSuccess(::take) }
    }

    fun refreshList() {
        scope.launch { runCatching { list = client.roomsList().items } }
    }

    /** The startup check: a round of ours still running opens as the room. */
    fun probe() {
        scope.launch { runCatching { client.roomsProbe() }.onSuccess(::take) }
    }

    fun make(mode: String, length: Int, autoStart: Boolean, then: () -> Unit) =
        open(then) { client.roomsMake(MakeRoomRequest(mode, length, autoStart)) }

    fun join(code: String, then: () -> Unit) = open(then) { client.roomsJoin(RoomCode(code)) }

    private fun open(then: () -> Unit, call: suspend () -> RoomView) {
        scope.launch {
            error = null
            note = null
            runCatching { call() }
                .onSuccess { v ->
                    opening = true
                    take(v)
                    then()
                }
                .onFailure { error = it }
        }
    }

    fun ready(on: Boolean) = move { client.roomsReady(ReadyRequest(on)) }

    fun start() = move { client.roomsStart() }

    fun rematch() = move { client.roomsRematch() }

    /** Steps away from the room; it keeps our seat. */
    fun close() {
        opening = false
        move { client.roomsClose() }
    }

    /** Leaves [code] for good, the open room or one in the list. */
    fun leave(code: String) {
        if (code == room.code) opening = false
        scope.launch {
            runCatching { client.roomsLeave(RoomCode(code)) }.onSuccess(::take)
            runCatching { list = client.roomsList().items }
        }
    }

    /** Asks for an any-time round's newer ranking; a room event brings it. */
    fun refreshResult() {
        val id = room.results?.id ?: return
        scope.launch {
            runCatching { client.roomsRefresh(IDRequest(id)) }.onFailure { error = it }
            refresh()
        }
    }

    fun clearNotes() {
        error = null
        note = null
    }

    private fun move(call: suspend () -> RoomView) {
        scope.launch { runCatching { call() }.onSuccess(::take).onFailure { error = it } }
    }

    private fun take(v: RoomView) {
        val was = room.active || opening
        room = v
        val b = v.board
        val held = board
        board = when {
            b == null -> null
            held != null && held.view.id == b.id -> held.also { it.update(b) }
            else -> RoomBoardModel(client, scope, b, onRoom = ::take)
        }
        val n = v.note
        val fresh = seen >= 0 && n.seq > seen
        seen = maxOf(seen, n.seq)
        if (fresh) {
            if (n.code in rowNotes) board?.refused(n.code) else note = n
        }
        // A make with no room yet is still on its way; a note is its answer.
        if (v.active || fresh) opening = false
        if (was && !v.active && !opening) onGone()
    }
}
