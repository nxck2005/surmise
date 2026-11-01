package com.nxck.surmise

import com.nxck.surmise.core.gen.SprintView
import com.nxck.surmise.sprint.SprintModel
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.UnconfinedTestDispatcher
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

// A sprint deals the next word once the solving row has turned, shows no
// result sheet, and ends in its summary when the clock runs out.
@OptIn(ExperimentalCoroutinesApi::class)
@RunWith(RobolectricTestRunner::class)
class SprintModelTest {
    private val running = SprintView(board = boardInPlay.copy(kind = "sprint"), length = 5, durationMs = 180_000, leftMs = 120_000, dealt = 1)
    private val solved = running.copy(board = boardWon.copy(kind = "sprint"), solved = 1)
    private val dealt = running.copy(board = boardInPlay.copy(id = "s-2", rows = emptyList(), typing = "", kind = "sprint"), solved = 1, dealt = 2)
    private val over = dealt.copy(over = true, leftMs = 0)

    private fun client(vararg answers: Pair<String, SprintView>) =
        fakeClient(answers.associate { (m, v) -> m to ok(SprintView.serializer(), v) })

    @Test
    fun theNextWordComesOnceTheRowHasTurned() = runTest(UnconfinedTestDispatcher()) {
        val m = SprintModel(client("sprint.submit" to solved, "sprint.next" to dealt), this, running) {}
        m.submit()
        assertEquals(1, m.run.solved)
        assertEquals("tweet", m.view.answer)
        m.revealed()
        assertEquals("s-2", m.view.id)
        assertEquals(2, m.run.dealt)
        assertNull(m.result)
        assertFalse(m.sheet)
    }

    @Test
    fun theClockRunningOutShowsTheSummary() = runTest(UnconfinedTestDispatcher()) {
        val m = SprintModel(client("sprint.view" to over), this, dealt) {}
        assertFalse(m.summary)
        m.tick()
        assertTrue(m.summary)
        assertTrue(m.run.over)
    }

    @Test
    fun stoppingEndsTheRun() = runTest(UnconfinedTestDispatcher()) {
        val m = SprintModel(client("sprint.end" to over), this, dealt) {}
        m.end()
        assertTrue(m.summary)
    }
}
