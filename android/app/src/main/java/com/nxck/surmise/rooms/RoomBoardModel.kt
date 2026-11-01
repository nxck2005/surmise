package com.nxck.surmise.rooms

import com.nxck.surmise.board.BoardModel
import com.nxck.surmise.board.Moves
import com.nxck.surmise.core.gen.BoardView
import com.nxck.surmise.core.gen.CoreClient
import com.nxck.surmise.core.gen.CoreException
import com.nxck.surmise.core.gen.RoomView
import com.nxck.surmise.core.gen.TypeRequest
import kotlinx.coroutines.CoroutineScope

/** A room round's keys go to rooms.*; the board is the room view's. */
private class RoomMoves(private val client: CoreClient, private val onRoom: (RoomView) -> Unit) : Moves {
    private fun board(v: RoomView): BoardView {
        onRoom(v)
        return v.board ?: throw CoreException("not_playing", "")
    }

    override suspend fun type(c: Char) = board(client.roomsType(TypeRequest(c.toString())))
    override suspend fun delete() = board(client.roomsDelete())

    // A room has no trim; a tap on a typed tile deletes back to it.
    override suspend fun trim(index: Int): BoardView {
        var v = client.roomsView()
        while ((v.board?.typing?.length ?: 0) > index) v = client.roomsDelete()
        return board(v)
    }

    override suspend fun submit() = board(client.roomsSubmit())
    override suspend fun view() = board(client.roomsView())
}

/**
 * Our board in a room round. The server marks a row, so a sent row's
 * marks arrive later, with a room event: [update] takes them and turns
 * the row then. A round has no result sheet of its own; the room's
 * results screen follows when the round ends.
 */
class RoomBoardModel(
    client: CoreClient,
    scope: CoroutineScope,
    initial: BoardView,
    onRoom: (RoomView) -> Unit,
) : BoardModel(client, scope, initial, {}, {}, RoomMoves(client, onRoom)) {
    /** A newer board from the room: turn a row that came back marked. */
    fun update(v: BoardView) {
        if (v == view) return
        val marked = v.rows.count { it.marks.isNotEmpty() }
        val had = view.rows.count { it.marks.isNotEmpty() }
        show(v)
        if (marked > had) revealing = marked - 1
    }

    /** A guess the server refused, which shakes the row like a local one. */
    fun refused(code: String) = refuse(CoreException(code, ""))

    override suspend fun finished() = Unit

    override fun landed() = Unit

    override suspend fun leave() = Unit
}
