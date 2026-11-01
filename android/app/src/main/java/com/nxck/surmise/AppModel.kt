package com.nxck.surmise

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import com.nxck.surmise.board.BoardModel
import com.nxck.surmise.core.CoreHandle
import com.nxck.surmise.core.gen.AppTokens
import com.nxck.surmise.core.gen.BoardView
import com.nxck.surmise.core.gen.ChallengeCode
import com.nxck.surmise.core.gen.CountRequest
import com.nxck.surmise.core.gen.CustomRequest
import com.nxck.surmise.core.gen.CalendarView
import com.nxck.surmise.core.gen.DailyOpenRequest
import com.nxck.surmise.core.gen.DailyView
import com.nxck.surmise.core.gen.DayRequest
import com.nxck.surmise.core.gen.DeleteRequest
import com.nxck.surmise.core.gen.HistoryItem
import com.nxck.surmise.core.gen.ImportReport
import com.nxck.surmise.core.gen.Info
import com.nxck.surmise.core.gen.LengthRequest
import com.nxck.surmise.core.gen.NewBoardRequest
import com.nxck.surmise.core.gen.OnlineView
import com.nxck.surmise.core.gen.ProfileView
import com.nxck.surmise.core.gen.ResumeRequest
import com.nxck.surmise.core.gen.Settings
import com.nxck.surmise.core.gen.SprintOptions
import com.nxck.surmise.core.gen.SprintRequest
import com.nxck.surmise.core.gen.SettingsPatch
import com.nxck.surmise.core.gen.ThemeInfo
import com.nxck.surmise.core.gen.TokensRequest
import com.nxck.surmise.core.gen.TrendsView
import com.nxck.surmise.design.SurmiseTokens
import com.nxck.surmise.home.Way
import com.nxck.surmise.modes.ChallengeState
import com.nxck.surmise.rooms.RoomsModel
import com.nxck.surmise.sprint.SprintModel
import com.nxck.surmise.settings.Appearance
import com.nxck.surmise.settings.Prefs
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch
import kotlinx.serialization.json.Json

/** A screen pushed over the tabs. The tabs are the root and never popped. */
sealed interface Route {
    data object Tabs : Route
    data object Daily : Route
    data object Year : Route
    data object HowTo : Route
    data object Sprint : Route
    data object Challenge : Route
    data object Custom : Route
    data object About : Route
    data object Rooms : Route
    data object Room : Route
    data object Board : Route
}

/** How deep a route sits, so a change of route knows whether it pushes or pops. */
val Route.depth: Int
    get() = when (this) {
        Route.Tabs -> 0
        Route.Daily -> 1
        Route.Year -> 1
        Route.HowTo -> 1
        Route.Sprint, Route.Challenge, Route.Custom -> 1
        Route.About -> 1
        Route.Rooms -> 1
        Route.Room -> 2
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
class AppModel(val core: CoreHandle, private val scope: CoroutineScope, val prefs: Prefs) {
    private val client get() = core.client

    var stack by mutableStateOf(listOf<Route>(Route.Tabs))
        private set
    var tab by mutableStateOf(Tab.Play)
        private set

    var settings by mutableStateOf(Settings())
        private set
    var daily by mutableStateOf<DailyView?>(null)
        private set
    var history by mutableStateOf(emptyList<HistoryItem>())
        private set
    var online by mutableStateOf(OnlineView())
        private set
    var profile by mutableStateOf<ProfileView?>(null)
        private set
    var calendar by mutableStateOf<CalendarView?>(null)
        private set
    var trends by mutableStateOf(emptyMap<Int, TrendsView>())
        private set

    /** The open board, while the board screen is up. */
    var board by mutableStateOf<BoardModel?>(null)
        private set

    /** A line for home when an opener fails ("that puzzle is gone"). */
    var homeNote by mutableStateOf<Throwable?>(null)

    val top: Route get() = stack.last()

    /** The room session; its screen is [Route.Room]. */
    val rooms = RoomsModel(core.client, scope, onGone = { if (top == Route.Room) pop() })

    init {
        scope.launch {
            core.events.collect { e ->
                when (e.kind) {
                    "online" -> refreshOnline()
                    "room" -> rooms.refresh()
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

    /**
     * The once-a-run reads: the server's status, and whether a round of ours
     * is still running, which home then offers to rejoin.
     */
    fun start() {
        scope.launch {
            runCatching { online = client.onlineStatus() }
            rooms.probe()
        }
    }

    /** Switches tab, reading what the record tab shows when it opens. */
    fun select(t: Tab) {
        tab = t
        if (t == Tab.Record) refreshRecord()
        if (t == Tab.Settings) refreshSettings()
    }

    /** Reads the profile, the calendar and each length's trends. */
    fun refreshRecord() {
        scope.launch {
            runCatching { profile = client.recordProfile() }
            runCatching { calendar = client.recordCalendar() }
            val t = mutableMapOf<Int, TrendsView>()
            for (n in 4..6) runCatching { t[n] = client.recordTrends(LengthRequest(n)) }
            trends = t
        }
    }

    /** Every theme the picker offers; read when the settings tab opens. */
    var themes by mutableStateOf(emptyList<ThemeInfo>())
        private set

    /** Tokens for a theme other than the bundled pair, by "name/tiles". */
    var themeTokens by mutableStateOf(emptyMap<String, SurmiseTokens>())
        private set

    /** The engine's version line, for about. */
    var info by mutableStateOf<Info?>(null)
        private set

    /** What the last backup or restore did, for a line under the buttons. */
    var backupNote by mutableStateOf<BackupNote?>(null)

    /**
     * The theme to draw: the chosen one, or its pair when the appearance
     * asks for the other side. "" chosen means the bundled night and day.
     */
    fun themeName(systemDark: Boolean): String = resolveTheme(prefs.theme, prefs.appearance, systemDark, themes)

    /** Asks the engine for [name]'s tokens with the chosen tiles, once each. */
    fun loadTokens(name: String) {
        val key = "$name/${prefs.tiles}"
        if (key in themeTokens) return
        scope.launch {
            runCatching { client.themesTokens(TokensRequest(name, prefs.tiles)) }
                .onSuccess { t -> themeTokens = themeTokens + (key to SurmiseTokens.fromJson(tokensJson.encodeToString(AppTokens.serializer(), t))) }
        }
    }

    /** Reads what the settings tab shows. */
    fun refreshSettings() {
        scope.launch {
            runCatching { settings = client.settingsGet() }
            runCatching { themes = client.themesList().items }
            runCatching { info = client.appInfo() }
        }
    }

    /** Changes engine settings; the answer is the settings as saved. */
    fun update(patch: SettingsPatch) {
        scope.launch {
            runCatching { settings = client.settingsUpdate(patch) }
            if (patch.network != null) refreshOnline()
        }
    }

    /** Writes the backup archive to [write], which the file picker opened. */
    fun exportBackup(write: (ByteArray) -> Unit) {
        scope.launch {
            backupNote = runCatching { write(core.exportBackup()) }
                .fold({ BackupNote.Saved }, { BackupNote.Failed })
        }
    }

    /** Merges the archive [read] gives into the install. */
    fun importBackup(read: () -> ByteArray) {
        scope.launch {
            backupNote = runCatching { core.importBackup(read()) }
                .fold({ BackupNote.Restored(it) }, { BackupNote.Failed })
            refreshHome()
            refreshSettings()
        }
    }

    fun show(route: Route) {
        if (top != route) stack = stack + route
    }

    /** Shows the year: the calendar and the trends. */
    fun showYear() {
        if (top != Route.Year) stack = stack + Route.Year
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
        val day = daily?.day ?: return
        if (!online.consent || !online.daily) return
        scope.launch { for (n in 4..6) runCatching { online = client.onlineDailyCount(CountRequest(day, n)) } }
    }

    /** Everyone's finished boards of today's daily, by length. */
    val dailyCounts: Map<Int, Int>
        get() = online.counts.filter { it.day == daily?.day }.associate { it.length to it.played }

    fun openDaily(length: Int) = open { client.dailyOpen(DailyOpenRequest(length = length)) }

    /** Deletes a saved puzzle. A deleted daily stays spent: it cannot be played again. */
    fun delete(id: String) {
        scope.launch {
            runCatching { client.historyDelete(DeleteRequest(id)) }
            refreshHome()
        }
    }

    fun resume(id: String) = open { client.boardResume(ResumeRequest(id)) }

    private fun open(onFail: (Throwable) -> Unit = { homeNote = it }, opener: suspend () -> BoardView) {
        scope.launch {
            homeNote = null
            runCatching { opener() }
                .onSuccess { v -> showBoard(boardModel(v)) }
                .onFailure(onFail)
        }
    }

    private fun showBoard(b: BoardModel) {
        board = b
        // A custom board takes the custom screen's place: back from it goes
        // home, never to a field that held the word.
        if (top == Route.Custom) stack = stack.dropLast(1)
        if (top != Route.Board) stack = stack + Route.Board
    }

    // --- other ways to play ---

    var sprintOptions by mutableStateOf(SprintOptions())
        private set
    var challenge by mutableStateOf(ChallengeState())
        private set
    var customNote by mutableStateOf<Throwable?>(null)
        private set

    /** The length a mode's setup starts on: the setting, or 5. */
    val startLength: Int get() = settings.length.takeIf { it in 4..6 } ?: 5

    fun showWay(w: Way) {
        when (w) {
            Way.Sprint -> {
                scope.launch { runCatching { sprintOptions = client.sprintOptions() } }
                show(Route.Sprint)
            }
            Way.Challenge -> {
                challenge = ChallengeState()
                show(Route.Challenge)
            }
            Way.Custom -> {
                customNote = null
                show(Route.Custom)
            }
            Way.Rooms -> {
                rooms.clearNotes()
                rooms.refreshList()
                show(Route.Rooms)
            }
        }
    }

    fun makeRoom(mode: String, length: Int, autoStart: Boolean) = rooms.make(mode, length, autoStart) { show(Route.Room) }

    fun joinRoom(code: String) = rooms.join(code) { show(Route.Room) }

    /** Opens the room a round of ours is still running in. */
    fun showRoom() {
        rooms.clearNotes()
        show(Route.Room)
    }

    /** Steps away from the room, keeping the seat. */
    fun awayFromRoom() {
        rooms.close()
        if (top == Route.Room) pop()
        rooms.refreshList()
    }

    /** Leaves the open room for good. */
    fun leaveRoom() {
        rooms.leave(rooms.room.code)
        if (top == Route.Room) pop()
    }

    fun startSprint(length: Int, durationMs: Long) {
        scope.launch {
            runCatching { client.sprintStart(SprintRequest(length, durationMs)) }
                .onSuccess { v -> showBoard(SprintModel(client, scope, v, onLeave = { closeBoard() })) }
                .onFailure { homeNote = it }
        }
    }

    fun makeChallenge(length: Int) {
        scope.launch {
            runCatching { client.challengeNew(LengthRequest(length)) }
                .onSuccess { challenge = ChallengeState(made = it.code, madeLength = length) }
                .onFailure { challenge = challenge.copy(note = it) }
        }
    }

    fun openChallenge(code: String) {
        challenge = challenge.copy(note = null)
        open(onFail = { challenge = challenge.copy(note = it) }) { client.challengeOpen(ChallengeCode(code)) }
    }

    /** Asks the engine whether [word] can be the answer, then [then] hands the phone over. */
    fun checkCustom(word: String, length: Int, anyWord: Boolean, then: () -> Unit) {
        scope.launch {
            customNote = null
            runCatching { client.customCheck(CustomRequest(word, length, anyWord)) }
                .onSuccess { then() }
                .onFailure { customNote = it }
        }
    }

    fun startCustom(word: String, length: Int, anyWord: Boolean) =
        open(onFail = { customNote = it }) { client.customStart(CustomRequest(word, length, anyWord)) }

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

    /**
     * Back on the board. A sprint still running stops and shows its summary,
     * as esc does in the terminal; the summary's own buttons, or back again,
     * leave it.
     */
    fun leaveBoard() {
        val b = board
        if (b is SprintModel && !b.run.over) b.end() else closeBoard()
    }

    /** Leaves the board for the play tab, whatever it was opened from. */
    fun goHome() {
        val b = board
        scope.launch {
            b?.leave()
            board = null
            stack = listOf(Route.Tabs)
            select(Tab.Play)
            refreshHome()
        }
    }

    fun back() {
        when (top) {
            Route.Board -> leaveBoard()
            Route.Room -> awayFromRoom()
            Route.Daily, Route.Year, Route.HowTo, Route.About, Route.Sprint, Route.Challenge, Route.Custom, Route.Rooms -> pop()
            Route.Tabs -> if (tab != Tab.Play) select(Tab.Play)
        }
    }

    /** Whether the back gesture is ours to handle, rather than the system's. */
    val handlesBack: Boolean get() = top != Route.Tabs || tab != Tab.Play

    private fun pop() {
        if (stack.size > 1) stack = stack.dropLast(1)
        refreshHome()
        if (tab == Tab.Record) refreshRecord()
    }

    private companion object {
        // The design side's decoder wants every field, so write them all.
        val tokensJson = Json { encodeDefaults = true }
    }

    /** The app went to the background: bank the running clock. */
    fun onStop() {
        board?.let { b -> scope.launch { b.leave() } }
    }
}

/** What the last backup or restore did. */
sealed interface BackupNote {
    data object Saved : BackupNote
    data class Restored(val report: ImportReport) : BackupNote
    data object Failed : BackupNote
}

/**
 * The theme to draw for [chosen] ("" is the bundled night and day): the
 * chosen theme, or its pair when [appearance] wants the other side.
 */
fun resolveTheme(chosen: String, appearance: Appearance, systemDark: Boolean, themes: List<ThemeInfo>): String {
    val dark = when (appearance) {
        Appearance.System -> systemDark
        Appearance.Light -> false
        Appearance.Dark -> true
    }
    if (chosen.isEmpty()) return if (dark) "night" else "day"
    val t = themes.firstOrNull { it.name == chosen } ?: return chosen
    return if (t.dark != dark && t.pair.isNotEmpty()) t.pair else chosen
}
