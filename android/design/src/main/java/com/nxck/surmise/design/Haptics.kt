package com.nxck.surmise.design

import android.os.Build
import android.view.HapticFeedbackConstants
import android.view.View
import androidx.compose.runtime.Composable
import androidx.compose.runtime.Stable
import androidx.compose.runtime.remember
import androidx.compose.ui.platform.LocalView

/**
 * The app's few haptics, through the view so they follow the system's touch
 * feedback setting. Each is a no-op when the player turned haptics off
 * ([SurmiseTheme]'s haptics flag).
 */
@Stable
class Haptics internal constructor(private val view: View, private val on: Boolean) {
    /** A tile lands in a flip, or a key is pressed: the lightest tick. */
    fun tick() = perform(HapticFeedbackConstants.CLOCK_TICK)

    /** A win: one firm confirm. */
    fun confirm() = perform(
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) HapticFeedbackConstants.CONFIRM else HapticFeedbackConstants.LONG_PRESS,
    )

    /** A guess the game refused (not a word, too short). */
    fun reject() = perform(
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) HapticFeedbackConstants.REJECT else HapticFeedbackConstants.LONG_PRESS,
    )

    private fun perform(constant: Int) {
        if (on) view.performHapticFeedback(constant)
    }
}

@Composable
fun rememberHaptics(): Haptics {
    val view = LocalView.current
    val on = LocalHapticsOn.current
    return remember(view, on) { Haptics(view, on) }
}
