package com.nxck.surmise

import com.nxck.surmise.board.BoardModel
import com.nxck.surmise.core.gen.BoardView
import com.nxck.surmise.core.gen.ProfileView
import com.nxck.surmise.core.gen.ResultView
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.UnconfinedTestDispatcher
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

// The board model's own rules: what a refusal does, when the result sheet
// comes up, and where next goes. The game's rules are the engine's and are
// tested in Go.
@OptIn(ExperimentalCoroutinesApi::class)
@RunWith(RobolectricTestRunner::class)
class BoardModelTest {
    @Test
    fun aWordNotInTheListShakesTheRowAndKeepsTheBoard() = runTest(UnconfinedTestDispatcher()) {
        val m = BoardModel(fakeClient(mapOf("board.submit" to refusal("not_a_word"))), this, boardInPlay, {}, {})
        m.submit()
        assertEquals(1, m.shakes)
        assertEquals(boardInPlay, m.view)
        assertEquals("not_a_word", (m.note as com.nxck.surmise.core.gen.CoreException).code)
    }

    @Test
    fun aFailedSaveShowsTheGuessButDoesNotShake() = runTest(UnconfinedTestDispatcher()) {
        val played = boardInPlay.copy(rows = boardWon.rows.take(3), typing = "")
        val m = BoardModel(
            fakeClient(mapOf("board.submit" to refusal("save_failed"), "board.view" to ok(BoardView.serializer(), played))),
            this, boardInPlay, {}, {},
        )
        m.submit()
        assertEquals(0, m.shakes)
        assertEquals(played, m.view)
        assertNotNull(m.note)
    }

    @Test
    fun theSheetWaitsForTheRowToTurn() = runTest(UnconfinedTestDispatcher()) {
        val m = BoardModel(
            fakeClient(
                mapOf(
                    "board.submit" to ok(BoardView.serializer(), boardWon),
                    "result.view" to ok(ResultView.serializer(), resultWon),
                    "record.profile" to ok(ProfileView.serializer(), profile),
                ),
            ),
            this, boardInPlay, {}, {},
        )
        m.submit()
        assertEquals(2, m.revealing)
        assertFalse(m.sheet)
        m.revealed()
        assertEquals(-1, m.revealing)
        assertTrue(m.sheet)
    }

    @Test
    fun nextOnARandomBoardDealsAFreshOne() = runTest(UnconfinedTestDispatcher()) {
        val fresh = boardInPlay.copy(id = "r-5-2", rows = emptyList(), typing = "")
        var swapped: BoardView? = null
        var home = false
        val m = BoardModel(
            fakeClient(
                mapOf(
                    "board.submit" to ok(BoardView.serializer(), boardWon),
                    "result.view" to ok(ResultView.serializer(), resultWon),
                    "board.new" to ok(BoardView.serializer(), fresh),
                ),
            ),
            this, boardInPlay, { home = true }, { swapped = it },
        )
        m.submit()
        m.next()
        assertEquals(fresh, swapped)
        assertFalse(home)
    }

    @Test
    fun nextOnADailyGoesToTheDailyScreen() = runTest(UnconfinedTestDispatcher()) {
        var left: String? = null
        val daily = boardWon.copy(kind = "daily")
        val m = BoardModel(
            fakeClient(
                mapOf(
                    "board.submit" to ok(BoardView.serializer(), daily),
                    "result.view" to ok(ResultView.serializer(), resultWon.copy(board = daily, next = "daily")),
                ),
            ),
            this, boardInPlay, { left = it }, {},
        )
        m.submit()
        m.next()
        assertEquals("daily", left)
    }
}
