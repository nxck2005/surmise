package com.nxck.surmise

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onRoot
import com.github.takahirom.roborazzi.captureRoboImage
import com.nxck.surmise.board.BoardModel
import com.nxck.surmise.board.BoardScreen
import com.nxck.surmise.board.NextAction
import com.nxck.surmise.board.ResultBody
import com.nxck.surmise.friends.FriendsActions
import com.nxck.surmise.friends.FriendsScreen
import com.nxck.surmise.friends.FriendsSheet
import com.nxck.surmise.friends.FriendsSheets
import com.nxck.surmise.friends.FriendsState
import com.nxck.surmise.history.HistoryScreen
import com.nxck.surmise.you.YouScreen
import java.time.LocalDate
import com.nxck.surmise.design.BundledTokens
import com.nxck.surmise.design.Motion
import com.nxck.surmise.design.Sheet
import com.nxck.surmise.design.SurmiseTheme
import com.nxck.surmise.home.HomeActions
import com.nxck.surmise.home.HomeScreen
import com.nxck.surmise.core.gen.SprintOptions
import com.nxck.surmise.core.gen.SprintView
import com.nxck.surmise.modes.ChallengeState
import com.nxck.surmise.modes.CustomScreen
import com.nxck.surmise.record.RecordScreen
import com.nxck.surmise.sprint.SprintSetup
import com.nxck.surmise.sprint.SprintSummaryBody
import com.nxck.surmise.record.YearScreen
import com.nxck.surmise.settings.HowToScreen
import com.nxck.surmise.settings.SettingsActions
import com.nxck.surmise.settings.SettingsScreen
import org.junit.Rule
import com.nxck.surmise.rooms.RoomActions
import com.nxck.surmise.rooms.RoomLines
import com.nxck.surmise.rooms.RoomResultScreen
import com.nxck.surmise.rooms.RoomScreen
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.ParameterizedRobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

// Screenshot tests compare whole frames with a committed baseline, because the
// rooms review showed that "this text is on screen" tests miss layout bugs.
// Record with ./gradlew :app:recordRoborazziDebug; CI runs verifyRoborazziDebug.
// Motion is off so each frame is the settled one.
@RunWith(ParameterizedRobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(qualifiers = "w411dp-h891dp-xxhdpi")
class ScreenScreenshotTest(private val theme: String) {
    companion object {
        @JvmStatic
        @ParameterizedRobolectricTestRunner.Parameters(name = "{0}")
        fun themes() = listOf(arrayOf<Any>("night"), arrayOf<Any>("day"))
    }

    @get:Rule val compose = createComposeRule()

    // Dates on screen follow the device's zone; pin it so CI (UTC) and a
    // developer's machine draw the same frame.
    @org.junit.Before
    fun utc() = java.util.TimeZone.setDefault(java.util.TimeZone.getTimeZone("UTC"))

    private fun shot(name: String, content: @Composable () -> Unit) {
        compose.setContent {
            val tokens = BundledTokens.load(LocalContext.current.assets, theme)
            SurmiseTheme(tokens = tokens, motion = Motion(enabled = false)) {
                Box(Modifier.fillMaxSize().background(SurmiseTheme.tokens.canvas)) { content() }
            }
        }
        compose.onRoot().captureRoboImage("src/test/screenshots/$name.$theme.png")
    }

    private val noHome = HomeActions(onDaily = {}, onPlay = {})

    @Test
    fun home() = shot("home") { HomeScreen(homeFixture, noHome) }

    @Test
    fun board() = shot("board") {
        val scope = rememberCoroutineScope()
        BoardScreen(BoardModel(fakeClient(emptyMap()), scope, boardInPlay, {}, {}), onBack = {}, onHome = {}, onShare = {})
    }

    @Test
    fun boardDaily() = shot("board-daily") {
        val scope = rememberCoroutineScope()
        val daily = boardInPlay.copy(kind = "daily", daily = "2026-11-01")
        BoardScreen(BoardModel(fakeClient(emptyMap()), scope, daily, {}, {}), onBack = {}, onHome = {}, onShare = {}, daily = dailyToday)
    }

    @Test
    fun boardOver() = shot("board-over") {
        val scope = rememberCoroutineScope()
        val won = boardWon.copy(kind = "daily", daily = "2026-11-01")
        val client = fakeClient(
            mapOf(
                "result.view" to ok(com.nxck.surmise.core.gen.ResultView.serializer(), resultWon.copy(board = won, next = "daily")),
                "record.profile" to ok(com.nxck.surmise.core.gen.ProfileView.serializer(), profile),
            ),
        )
        val m = androidx.compose.runtime.remember { BoardModel(client, scope, won, {}, {}) }
        androidx.compose.runtime.LaunchedEffect(m) {
            m.loadOpened()
            m.sheet = true
        }
        BoardScreen(m, onBack = {}, onHome = {}, onShare = {}, daily = dailyToday)
    }

    @Test
    fun homeDailyDone() = shot("home-daily-done") {
        val done = dailyToday.copy(
            resetsAtMs = 1_000L + 33_120_000L,
            rows = dailyToday.rows.map { it.copy(state = "won", attempts = 3, elapsedMs = 58_000, marks = List(3) { _ -> List(it.length) { "correct" } }) },
            trio = com.nxck.surmise.core.gen.Trio(done = 3, of = 3, guesses = 9, elapsedMs = 245_000, complete = true),
            share = "surmise daily",
        )
        HomeScreen(homeFixture.copy(daily = done, inPlay = emptyMap(), dailyCounts = mapOf(4 to 312, 5 to 1284, 6 to 201)), noHome, now = { 1_000L })
    }

    @Test
    fun friends() = shot("friends") { FriendsScreen(FriendsState(rooms = true, entries = roomEntries), FriendsActions()) }

    @Test
    fun friendsOffline() = shot("friends-offline") { FriendsScreen(FriendsState(rooms = false, offline = true), FriendsActions()) }

    @Test
    fun makeRoom() = shot("make-room") {
        FriendsSheets(FriendsSheet.Room, 5, null, ChallengeState(), {}, { _, _, _ -> }, {}, {}, {})
    }

    @Test
    fun history() = shot("history") {
        YouScreen(profile, history = true, {}, {}, record = {}, list = { HistoryScreen(historyItems, {}, {}, today = shotDay) })
    }

    @Test
    fun historyDay() = shot("history-day") {
        YouScreen(profile, history = true, {}, {}, record = {}, list = {
            HistoryScreen(historyItems, {}, {}, day = "2026-11-01", today = shotDay)
        })
    }

    @Test
    fun result() = shot("result") {
        Box(Modifier.fillMaxSize(), contentAlignment = Alignment.BottomCenter) {
            Sheet(visible = true, onDismiss = {}) {
                ResultBody(resultWon, profile, onShare = {}, onDone = {}, next = NextAction("next word") {})
            }
        }
    }

    @Test
    fun record() = shot("record") {
        YouScreen(profile, history = false, {}, {}, record = { RecordScreen(profile, calendarFixture, onYear = {}) }, list = {})
    }

    @Test
    fun year() = shot("year") { YearScreen(calendarFixture, trendsFixture, onBack = {}) }

    @Test
    fun settings() = shot("settings") {
        val none = SettingsActions({}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {})
        SettingsScreen(settingsFixture, none, onBack = {})
    }

    @Test
    fun howTo() = shot("howto") { HowToScreen(onBack = {}) }

    @Test
    fun sprintSetup() = shot("sprint-setup") {
        SprintSetup(SprintOptions(listOf(10_000, 15_000, 30_000, 60_000, 120_000, 300_000, 600_000), 300_000), 5, { _, _ -> }, {})
    }

    @Test
    fun sprintSummary() = shot("sprint-summary") {
        Sheet(visible = true, onDismiss = {}) {
            SprintSummaryBody(
                SprintView(length = 5, durationMs = 180_000, over = true, dealt = 9, solved = 7, missed = 2, avgGuesses = 3.9, accuracy = 77, solveTimeMs = 147_000),
                {}, {},
            )
        }
    }

    @Test
    fun challenge() = shot("challenge") {
        FriendsSheets(FriendsSheet.Challenge, 5, null, ChallengeState(made = "7K2M-QX4R-9TPW-B3HD", madeLength = 5), {}, { _, _, _ -> }, {}, {}, {})
    }

    @Test
    fun custom() = shot("custom") { CustomScreen(5, null, { _, _, _, _ -> }, { _, _, _ -> }, {}) }

    @Test
    fun roomLobby() = shot("room-lobby") { RoomScreen(roomLobby, RoomLines(), RoomActions(), now = { 0 }) }

    @Test
    fun roomWatch() = shot("room-watch") { RoomScreen(roomWatch, RoomLines(), RoomActions(), now = { 1_000_000 }) }

    @Test
    fun roomResult() = shot("room-result") { RoomScreen(roomResult, RoomLines(), RoomActions(), now = { 0 }) }

    @Test
    fun roomResultSaved() = shot("room-result-saved") {
        RoomResultScreen(roomResult.results!!.copy(mode = "async", canRematch = false, canRefresh = true), RoomLines(), RoomActions())
    }
}

/** The day the history shots call today, so the headings never move. */
private val shotDay: LocalDate = LocalDate.of(2026, 11, 1)
