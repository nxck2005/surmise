package com.nxck.surmise

import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onRoot
import com.github.takahirom.roborazzi.captureRoboImage
import com.nxck.surmise.design.SurmiseTheme
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

// Screenshot tests compare whole frames with a committed baseline, because the
// rooms review showed that "this text is on screen" tests miss layout bugs.
// Record with ./gradlew recordRoborazziDebug; CI runs verifyRoborazziDebug.
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(qualifiers = "w411dp-h891dp-xxhdpi")
class PlaceholderScreenshotTest {
    @get:Rule val compose = createComposeRule()

    @Test
    fun placeholder() {
        compose.setContent { SurmiseTheme { Placeholder() } }
        compose.onRoot().captureRoboImage()
    }
}
