package com.nxck.surmise

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import com.nxck.surmise.board.BoardModel
import com.nxck.surmise.core.CoreHandle
import com.nxck.surmise.core.gen.BoardView
import com.nxck.surmise.core.gen.DailyOpenRequest
import com.nxck.surmise.core.gen.DailyView
import com.nxck.surmise.core.gen.DayRequest
import com.nxck.surmise.core.gen.DeleteRequest
import com.nxck.surmise.core.gen.HistoryItem
import com.nxck.surmise.core.gen.NewBoardRequest
import com.nxck.surmise.core.gen.OnlineView
import com.nxck.surmise.core.gen.ResumeRequest
import com.nxck.surmise.core.gen.Settings
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch

/** A screen pushed over the tabs. The tabs are the root and never popped. */
sealed interface Route {
    data object Tabs : Route
    data object Daily : Route
    data object Board : Route
}

/** How deep a route sits, so a change of route knows whether it pushes or pops. */
val Route.depth: Int
    get() = when (this) {
        Route.Tabs -> 0
        Route.Daily -> 1
        Route.Board -> 2
    }

enum class Tab { Play, History, Record, Settings }

/**
 * What the app shows, held as Compose state. Every change goes through the
 * engine: this class asks, and keeps the answer; it decides no rule of the
 * game itself.
 *
 * [scope] runs on the main thread. The engine's calls all queue on one
 * thread of their own, so calls made in order are answered in order.
 */
class AppModel(val core: CoreHandle, private val scope: CoroutineScope) {
    private val client get() = core.client

    var stack by mutableStateOf(listOf<Route>(Route.Tabs))
        private set
    var tab by mutableStateOf(Tab.Play)

    var settings by mutableStateOf(Settings())
        private set
    var daily by mutableStateOf<DailyView?>(null)
        private set
    var history by mutableStateOf(emptyList<HistoryItem>())
        private set
    var online by mutableStateOf(OnlineView())
        private set

    /** The open board, while the board screen is up. */
    var board by mutableStateOf<BoardModel?>(null)
        private set

    /** A line for home when an opener fails ("that puzzle is gone"). */
    var homeNote by mutableStateOf<Throwable?>(null)

    val top: Route get() = stack.last()

    init {
        scope.launch {
            core.events.collect { e ->
                when (e.kind) {
                    "online" -> refreshOnline()
                }
            }
        }
    }

    /** Reads everything home shows. Called on start and on each return home. */
    fun refreshHome() {
        scope.launch {
            runCatching { settings = client.settingsGet() }
            runCatching { daily = client.dailyView(DayRequest()) }
            runCatching { history = client.historyList().items }
            runCatching { online = client.onlineMenuCount() }
        }
    }

    private suspend fun refreshOnline() {
        runCatching { online = client.onlineView() }
    }

    /** The newest random board of [length] still in play, for its play card. */
    fun inPlay(length: Int): HistoryItem? =
        history.firstOrNull { it.kind == "random" && it.length == length && it.status == "in_progress" }

    fun playLength(length: Int) {
        val resume = inPlay(length)
        open {
            if (resume != null) client.boardResume(ResumeRequest(resume.id)) else client.boardNew(NewBoardRequest(length))
        }
    }

    /** Shows the daily screen. */
    fun showDaily() {
        if (top != Route.Daily) stack = stack + Route.Daily
        refreshHome()
    }

    fun openDaily(length: Int) = open { client.dailyOpen(DailyOpenRequest(length = length)) }

    /** Deletes a saved puzzle. A deleted daily stays spent: it cannot be played again. */
    fun delete(id: String) {
        scope.launch {
            runCatching { client.historyDelete(DeleteRequest(id)) }
            refreshHome()
        }
    }

    fun resume(id: String) = open { client.boardResume(ResumeRequest(id)) }

    private fun open(opener: suspend () -> BoardView) {
        scope.launch {
            homeNote = null
            runCatching { opener() }
                .onSuccess { v ->
                    board = boardModel(v)
                    if (top != Route.Board) stack = stack + Route.Board
                }
                .onFailure { homeNote = it }
        }
    }

    /** Swaps the open board for another (next puzzle, play again). */
    private fun replaceBoard(v: BoardView) {
        board = boardModel(v)
    }

    private fun boardModel(v: BoardView) = BoardModel(
        client,
        scope,
        v,
        onLeave = { next -> closeBoard(then = if (next == "daily") Route.Daily else null) },
        onReplace = ::replaceBoard,
    )

    /**
     * Leaves the board screen: banks the clock and saves, then goes back, or
     * on to [then] (the daily screen, after a daily's result).
     */
    fun closeBoard(then: Route? = null) {
        val b = board
        scope.launch {
            b?.leave()
            board = null
            pop()
            if (then != null && top != then) stack = stack + then
        }
    }

    /** Leaves the board for the play tab, whatever it was opened from. */
    fun goHome() {
        val b = board
        scope.launch {
            b?.leave()
            board = null
            stack = listOf(Route.Tabs)
            tab = Tab.Play
            refreshHome()
        }
    }

    fun back() {
        when (top) {
            Route.Board -> closeBoard()
            Route.Daily -> pop()
            Route.Tabs -> if (tab != Tab.Play) tab = Tab.Play
        }
    }

    /** Whether the back gesture is ours to handle, rather than the system's. */
    val handlesBack: Boolean get() = top != Route.Tabs || tab != Tab.Play

    private fun pop() {
        if (stack.size > 1) stack = stack.dropLast(1)
        refreshHome()
    }

    /** The app went to the background: bank the running clock. */
    fun onStop() {
        board?.let { b -> scope.launch { b.leave() } }
    }
}
