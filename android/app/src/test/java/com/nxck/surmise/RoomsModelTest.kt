package com.nxck.surmise

import com.nxck.surmise.core.gen.CoreClient
import com.nxck.surmise.core.gen.Note
import com.nxck.surmise.core.gen.RoomView
import com.nxck.surmise.core.gen.coreJson
import com.nxck.surmise.rooms.RoomsModel
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.UnconfinedTestDispatcher
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

// The room session follows the engine's view: a make waits for its room, a
// failed join comes back as a note and leaves the room screen, and a word
// the server refuses shakes the board instead of showing a line.
@OptIn(ExperimentalCoroutinesApi::class)
@RunWith(RobolectricTestRunner::class)
class RoomsModelTest {
    /** A client whose rooms.view answer is whatever [view] holds now. */
    private class Rooms(var view: RoomView = RoomView()) {
        val client = CoreClient { method, _ ->
            when (method) {
                "rooms.view", "rooms.make", "rooms.join", "rooms.probe", "rooms.submit", "rooms.close" ->
                    """{"ok":${coreJson.encodeToString(RoomView.serializer(), view)}}"""
                else -> refusal("unknown_method")
            }
        }
    }

    @Test
    fun aMakeWaitsForItsRoom() = runTest(UnconfinedTestDispatcher()) {
        val r = Rooms()
        var gone = 0
        val m = RoomsModel(r.client, this) { gone++ }
        m.probe()
        var shown = false
        m.make("live", 5, true) { shown = true }
        assertTrue(shown)
        assertTrue(m.opening)
        r.view = roomLobby
        m.refresh()
        assertFalse(m.opening)
        assertEquals("lobby", m.room.screen)
        assertEquals(0, gone)
    }

    @Test
    fun aFailedJoinLeavesWithItsNote() = runTest(UnconfinedTestDispatcher()) {
        val r = Rooms(RoomView(note = Note("left", "OLD111", 3)))
        var gone = 0
        val m = RoomsModel(r.client, this) { gone++ }
        m.probe()
        // The old note was there before: it shows no line.
        assertNull(m.note)
        m.join("KVR7QD") {}
        r.view = RoomView(note = Note("no_room", "KVR7QD", 4))
        m.refresh()
        assertEquals("no_room", m.note?.code)
        assertEquals(1, gone)
    }

    @Test
    fun aRefusedWordShakesTheBoard() = runTest(UnconfinedTestDispatcher()) {
        val playing = roomLobby.copy(screen = "board", phase = "playing", board = boardInPlay.copy(kind = "room"))
        val r = Rooms(playing)
        val m = RoomsModel(r.client, this) {}
        m.refresh()
        val b = assertNotNullAnd(m.board)
        r.view = playing.copy(note = Note("not_a_word", "KVR7QD", 1))
        m.refresh()
        assertEquals(1, b.shakes)
        assertNull(m.note)
    }

    @Test
    fun aMarkedRowTurnsWhenItLands() = runTest(UnconfinedTestDispatcher()) {
        val sent = roomLobby.copy(screen = "board", phase = "playing", pending = true, board = boardInPlay.copy(kind = "room", typing = "tweet"))
        val r = Rooms(sent)
        val m = RoomsModel(r.client, this) {}
        m.refresh()
        val b = assertNotNullAnd(m.board)
        assertEquals(-1, b.revealing)
        r.view = sent.copy(pending = false, board = boardWon.copy(kind = "room"))
        m.refresh()
        assertEquals(b, m.board)
        assertEquals(2, b.revealing)
    }

    private fun <T> assertNotNullAnd(v: T?): T {
        assertNotNull(v)
        return v!!
    }
}
