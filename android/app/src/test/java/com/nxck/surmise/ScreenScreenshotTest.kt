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
import com.nxck.surmise.design.BundledTokens
import com.nxck.surmise.design.Motion
import com.nxck.surmise.design.Sheet
import com.nxck.surmise.design.SurmiseTheme
import com.nxck.surmise.home.HomeActions
import com.nxck.surmise.home.HomeScreen
import org.junit.Rule
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

    private fun shot(name: String, content: @Composable () -> Unit) {
        compose.setContent {
            val tokens = BundledTokens.load(LocalContext.current.assets, theme)
            SurmiseTheme(tokens = tokens, motion = Motion(enabled = false)) {
                Box(Modifier.fillMaxSize().background(SurmiseTheme.tokens.canvas)) { content() }
            }
        }
        compose.onRoot().captureRoboImage("src/test/screenshots/$name.$theme.png")
    }

    private val noHome = HomeActions(onDaily = {}, onPlay = {}, onRecord = {})

    @Test
    fun home() = shot("home") { HomeScreen(homeFixture, noHome) }

    @Test
    fun board() = shot("board") {
        val scope = rememberCoroutineScope()
        BoardScreen(BoardModel(fakeClient(emptyMap()), scope, boardInPlay, {}, {}), onBack = {}, onShare = {})
    }

    @Test
    fun result() = shot("result") {
        Box(Modifier.fillMaxSize(), contentAlignment = Alignment.BottomCenter) {
            Sheet(visible = true, onDismiss = {}) {
                ResultBody(resultWon, profile, onShare = {}, onReview = {}, onNext = {}, onHome = {})
            }
        }
    }
}
