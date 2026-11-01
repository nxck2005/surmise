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
import com.nxck.surmise.board.ResultBody
import com.nxck.surmise.daily.DailyScreen
import com.nxck.surmise.history.HistoryScreen
import com.nxck.surmise.design.BundledTokens
import com.nxck.surmise.design.Motion
import com.nxck.surmise.design.Sheet
import com.nxck.surmise.design.SurmiseTheme
import com.nxck.surmise.home.HomeActions
import com.nxck.surmise.home.HomeScreen
import com.nxck.surmise.core.gen.SprintOptions
import com.nxck.surmise.core.gen.SprintView
import com.nxck.surmise.modes.ChallengeScreen
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
import com.nxck.surmise.rooms.RoomScreen
import com.nxck.surmise.rooms.RoomsScreen
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

    private val noHome = HomeActions(onDaily = {}, onDailyScreen = {}, onPlay = {}, onRecord = {})

    @Test
    fun home() = shot("home") { HomeScreen(homeFixture, noHome) }

    @Test
    fun board() = shot("board") {
        val scope = rememberCoroutineScope()
        BoardScreen(BoardModel(fakeClient(emptyMap()), scope, boardInPlay, {}, {}), onBack = {}, onHome = {}, onShare = {})
    }

    @Test
    fun daily() = shot("daily") {
        DailyScreen(dailyToday.copy(resetsAtMs = 1_000L + 33_120_000L), "sunday, november 1", 1284, {}, {}, {}, now = { 1_000L })
    }

    @Test
    fun dailyComplete() = shot("daily-complete") {
        val done = dailyToday.copy(
            rows = dailyToday.rows.map { it.copy(state = "won", attempts = 3, elapsedMs = 58_000, marks = List(3) { _ -> List(it.length) { "correct" } }) },
            trio = com.nxck.surmise.core.gen.Trio(done = 3, of = 3, guesses = 9, elapsedMs = 245_000, complete = true),
            share = "surmise daily",
        )
        DailyScreen(done, "sunday, november 1", 0, {}, {}, {}, now = { 1_000L })
    }

    @Test
    fun history() = shot("history") { HistoryScreen(historyItems, {}, {}) }

    @Test
    fun result() = shot("result") {
        Box(Modifier.fillMaxSize(), contentAlignment = Alignment.BottomCenter) {
            Sheet(visible = true, onDismiss = {}) {
                ResultBody(resultWon, profile, onShare = {}, onReview = {}, onNext = {}, onHome = {})
            }
        }
    }

    @Test
    fun record() = shot("record") { RecordScreen(profile, calendarFixture, onYear = {}) }

    @Test
    fun year() = shot("year") { YearScreen(calendarFixture, trendsFixture, onBack = {}) }

    @Test
    fun settings() = shot("settings") {
        val none = SettingsActions({}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {})
        SettingsScreen(settingsFixture, none)
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
        ChallengeScreen(ChallengeState(made = "7K2M-QX4R-9TPW-B3HD", madeLength = 5), 5, {}, {}, {}, {})
    }

    @Test
    fun custom() = shot("custom") { CustomScreen(5, null, { _, _, _, _ -> }, { _, _, _ -> }, {}) }

    @Test
    fun rooms() = shot("rooms") { RoomsScreen(roomEntries, RoomLines(), 5, { _, _, _ -> }, {}, {}, {}) }

    @Test
    fun roomLobby() = shot("room-lobby") { RoomScreen(roomLobby, RoomLines(), RoomActions(), now = { 0 }) }

    @Test
    fun roomWatch() = shot("room-watch") { RoomScreen(roomWatch, RoomLines(), RoomActions(), now = { 1_000_000 }) }

    @Test
    fun roomResult() = shot("room-result") { RoomScreen(roomResult, RoomLines(), RoomActions(), now = { 0 }) }
}
