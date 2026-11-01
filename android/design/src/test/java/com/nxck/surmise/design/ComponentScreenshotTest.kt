package com.nxck.surmise.design

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.unit.dp
import com.github.takahirom.roborazzi.captureRoboImage
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.ParameterizedRobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

// Every component, in night and day. Record with
// ./gradlew :design:recordRoborazziDebug; CI runs verifyRoborazziDebug.
// Motion is off so each frame is the settled one.
@RunWith(ParameterizedRobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(qualifiers = "w411dp-h891dp-xxhdpi")
class ComponentScreenshotTest(private val theme: String) {
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
                Box(
                    Modifier
                        .testTag(SHOT)
                        .background(SurmiseTheme.tokens.canvas)
                        .padding(16.dp),
                ) { content() }
            }
        }
        compose.onNodeWithTag(SHOT).captureRoboImage("src/test/screenshots/$name.$theme.png")
    }

    @Test
    fun type() = shot("type") {
        val t = SurmiseTheme.type
        val ink = SurmiseTheme.tokens.ink
        val dim = SurmiseTheme.tokens.inkDim
        Column(Modifier.width(360.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            SurmiseText("surmise", t.display, ink)
            SurmiseText("got it in four", t.displaySmall, ink)
            SurmiseText("today's three", t.title, ink)
            SurmiseText("settings", t.headline, ink)
            SurmiseText("play", t.section, ink)
            SurmiseText("a new word every day, the same for everyone.", t.body, dim)
            SurmiseText("sunday, 1 november", t.caption, dim)
            SurmiseText("resets at midnight", t.footnote, SurmiseTheme.tokens.inkMuted)
            SurmiseText("C5-7Q2K-XM", t.code, dim)
            SurmiseText("02:41", t.clock, ink)
        }
    }

    @Test
    fun card() = shot("card") {
        Card(Modifier.width(360.dp)) {
            Column {
                SurmiseText("classic", SurmiseTheme.type.section, SurmiseTheme.tokens.ink)
                SurmiseText("six tries, five letters", SurmiseTheme.type.caption, SurmiseTheme.tokens.inkDim)
            }
        }
    }

    @Test
    fun buttons() = shot("buttons") {
        Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            CapsuleButton("play", {})
            CapsuleButton("share", {}, style = CapsuleStyle.Secondary)
            CapsuleButton("play", {}, enabled = false)
        }
    }

    @Test
    fun switches() = shot("switches") {
        Row(horizontalArrangement = Arrangement.spacedBy(16.dp)) {
            Switch(checked = true, onCheckedChange = {})
            Switch(checked = false, onCheckedChange = {})
        }
    }

    @Test
    fun tiles() = shot("tiles") {
        Column(verticalArrangement = Arrangement.spacedBy(TileGap)) {
            Row(horizontalArrangement = Arrangement.spacedBy(TileGap)) {
                Tile('s', TileState.Correct)
                Tile('l', TileState.Absent)
                Tile('a', TileState.Present)
                Tile('t', TileState.Absent)
                Tile('e', TileState.Correct)
            }
            Row(horizontalArrangement = Arrangement.spacedBy(TileGap)) {
                Tile('c', TileState.Typed)
                Tile('o', TileState.Typed)
                Tile(null, TileState.Cursor)
                Tile(null, TileState.Ready)
                Tile(null, TileState.Ready)
            }
            Row(horizontalArrangement = Arrangement.spacedBy(TileGap)) {
                repeat(5) { Tile(null, TileState.Empty) }
            }
        }
    }

    @Test
    fun keyboard() = shot("keyboard") {
        Box(Modifier.width(390.dp)) {
            Keyboard(
                states = mapOf(
                    's' to KeyState.Correct,
                    'e' to KeyState.Correct,
                    'a' to KeyState.Present,
                    'l' to KeyState.Spent,
                    't' to KeyState.Spent,
                ),
                onLetter = {},
                onEnter = {},
                onDelete = {},
            )
        }
    }

    @Test
    fun icons() = shot("icons") {
        val ink = SurmiseTheme.tokens.ink
        val all = listOf(
            SurmiseIcons.Play, SurmiseIcons.History, SurmiseIcons.Record, SurmiseIcons.Settings,
            SurmiseIcons.Back, SurmiseIcons.Chevron, SurmiseIcons.ArrowRight, SurmiseIcons.Backspace,
            SurmiseIcons.Custom, SurmiseIcons.Sprint, SurmiseIcons.Rooms, SurmiseIcons.Check,
            SurmiseIcons.Share, SurmiseIcons.Download, SurmiseIcons.Restart, SurmiseIcons.Theme,
            SurmiseIcons.Info, SurmiseIcons.Help, SurmiseIcons.Globe,
        )
        Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
            all.chunked(7).forEach { row ->
                Row(horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                    row.forEach { Icon(it, ink, size = 24.dp) }
                }
            }
        }
    }

    @Test
    fun tabBar() = shot("tabbar") {
        Box(Modifier.width(390.dp).padding(horizontal = 4.dp)) {
            TabBar(
                items = listOf(
                    TabItem("play", SurmiseIcons.Play),
                    TabItem("history", SurmiseIcons.History),
                    TabItem("record", SurmiseIcons.Record),
                    TabItem("settings", SurmiseIcons.Settings),
                ),
                selected = 0,
                onSelect = {},
            )
        }
    }

    @Test
    fun meshGlow() = shot("glow") {
        MeshGlow(
            Modifier
                .width(360.dp)
                .height(200.dp)
                .padding(0.dp),
        ) {
            Column(Modifier.padding(22.dp)) {
                SurmiseText("daily", SurmiseTheme.type.caption, SurmiseTheme.tokens.inkDim)
                SurmiseText("today's three", SurmiseTheme.type.title, SurmiseTheme.tokens.ink)
            }
        }
    }

    @Test
    fun sheet() = shot("sheet") {
        Box(Modifier.size(390.dp, 420.dp)) {
            Column(Modifier.padding(16.dp)) {
                SurmiseText("behind the sheet", SurmiseTheme.type.title, SurmiseTheme.tokens.ink)
            }
            Sheet(visible = true, onDismiss = {}) {
                SurmiseText("share result", SurmiseTheme.type.headline, SurmiseTheme.tokens.ink)
                Box(Modifier.height(12.dp))
                SurmiseText("copies your grid, without the letters.", SurmiseTheme.type.body, SurmiseTheme.tokens.inkDim)
                Box(Modifier.height(20.dp))
                Box(Modifier.fillMaxWidth(), contentAlignment = Alignment.Center) {
                    CapsuleButton("copy", {})
                }
            }
        }
    }
}

private const val SHOT = "shot"
