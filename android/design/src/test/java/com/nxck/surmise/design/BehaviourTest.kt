package com.nxck.surmise.design

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.width
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshots.Snapshot
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.unit.dp
import com.github.takahirom.roborazzi.captureRoboImage
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(qualifiers = "w411dp-h891dp-xxhdpi")
class BehaviourTest {
    @get:Rule val compose = createComposeRule()

    private val context get() = RuntimeEnvironment.getApplication()

    /**
     * A theme changes colours and nothing else: every component lands in the
     * same place, at the same size, in night and in day.
     */
    @Test
    fun themeSwapMovesNothing() {
        var tokens by mutableStateOf(BundledTokens.night(context))
        val tags = listOf("card", "button", "switch", "tile", "keyboard", "tabbar", "text")
        compose.setContent {
            SurmiseTheme(tokens = tokens) {
                Column(Modifier.width(390.dp)) {
                    Card(Modifier.testTag("card")) {
                        SurmiseText("classic", SurmiseTheme.type.section, SurmiseTheme.tokens.ink, Modifier.testTag("text"))
                    }
                    CapsuleButton("play", {}, Modifier.testTag("button"))
                    Switch(true, {}, Modifier.testTag("switch"))
                    Tile('a', TileState.Present, Modifier.testTag("tile"))
                    Keyboard(emptyMap(), {}, {}, {}, Modifier.testTag("keyboard"))
                    TabBar(listOf(TabItem("play", SurmiseIcons.Play), TabItem("record", SurmiseIcons.Record)), 0, {}, Modifier.testTag("tabbar"))
                }
            }
        }
        fun bounds(): Map<String, Rect> = tags.associateWith { compose.onNodeWithTag(it, useUnmergedTree = true).fetchSemanticsNode().boundsInRoot }
        val night = bounds()
        tokens = BundledTokens.day(context)
        compose.mainClock.advanceTimeBy(THEME_FADE_MS * 2L)
        assertEquals(night, bounds())
    }

    /** With motion off a flip lands at once, and still says so. */
    @Test
    fun flipWithoutMotionStillReports() {
        var state by mutableStateOf(TileState.Typed)
        var flipped = 0
        compose.setContent {
            SurmiseTheme(tokens = BundledTokens.night(LocalContext.current), motion = Motion(enabled = false)) {
                Tile('a', state, onFlipped = { flipped++ })
            }
        }
        compose.runOnIdle { state = TileState.Correct }
        compose.waitForIdle()
        assertEquals(1, flipped)
    }

    /** With motion on, the flip waits out its delay and reports once it lands. */
    @Test
    fun flipReportsWhenItLands() {
        var state by mutableStateOf(TileState.Typed)
        var flipped = 0
        compose.mainClock.autoAdvance = false
        compose.setContent {
            SurmiseTheme(tokens = BundledTokens.night(LocalContext.current)) {
                Tile('a', state, flipDelayMs = FLIP_STAGGER_MS, onFlipped = { flipped++ })
            }
        }
        compose.mainClock.advanceTimeByFrame()
        state = TileState.Correct
        Snapshot.sendApplyNotifications()
        compose.mainClock.advanceTimeByFrame()
        compose.mainClock.advanceTimeBy((FLIP_STAGGER_MS + FLIP_MS / 4).toLong())
        assertEquals(0, flipped)
        compose.mainClock.advanceTimeBy(FLIP_MS * 2L)
        assertEquals(1, flipped)
    }

    /** A frame from the middle of a row's flip, to see the turn and the stagger. */
    @Test
    fun flipMidway() {
        var states by mutableStateOf(List(5) { TileState.Typed })
        compose.mainClock.autoAdvance = false
        compose.setContent {
            SurmiseTheme(tokens = BundledTokens.night(LocalContext.current)) {
                Row(Modifier.testTag("row").background(SurmiseTheme.tokens.canvas), horizontalArrangement = Arrangement.spacedBy(TileGap)) {
                    "slate".forEachIndexed { i, c -> Tile(c, states[i], flipDelayMs = i * FLIP_STAGGER_MS) }
                }
            }
        }
        compose.mainClock.advanceTimeBy(1000)
        states = listOf(TileState.Correct, TileState.Absent, TileState.Present, TileState.Absent, TileState.Correct)
        Snapshot.sendApplyNotifications()
        compose.mainClock.advanceTimeByFrame()
        compose.mainClock.advanceTimeBy(400)
        compose.onNodeWithTag("row").captureRoboImage("src/test/screenshots/tiles-flip-midway.night.png")
    }

    @Test
    fun switchToggles() {
        var on by mutableStateOf(false)
        compose.setContent {
            SurmiseTheme(tokens = BundledTokens.night(LocalContext.current)) {
                Switch(on, { on = it }, Modifier.testTag("switch"))
            }
        }
        compose.onNodeWithTag("switch").performClick()
        assertTrue(on)
    }

    @Test
    fun keyboardTypes() {
        val typed = StringBuilder()
        compose.setContent {
            SurmiseTheme(tokens = BundledTokens.night(LocalContext.current)) {
                Keyboard(emptyMap(), { typed.append(it) }, { typed.append('+') }, { typed.append('-') })
            }
        }
        compose.onNodeWithText("Q").performClick()
        compose.onNodeWithText("enter").performClick()
        compose.onNodeWithContentDescription("delete").performClick()
        assertEquals("q+-", typed.toString())
    }
}
