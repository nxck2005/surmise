package com.nxck.surmise.settings

import android.content.Context
import android.content.SharedPreferences
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue

/** Light, dark, or as the system is. */
enum class Appearance { System, Light, Dark }

/**
 * What only the phone keeps: the look and the haptics. The engine has no
 * field for these, so they live in the app's own preferences, and a backup
 * does not carry them.
 */
class Prefs(private val sp: SharedPreferences) {
    /** The chosen theme's name, or "" for the bundled night and day. */
    var theme by mutableStateOf(sp.getString(THEME, "") ?: "")
        private set
    var appearance by mutableStateOf(
        runCatching { Appearance.valueOf(sp.getString(APPEARANCE, "") ?: "") }.getOrDefault(Appearance.System),
    )
        private set

    /** "", "colorblind" or "high-contrast", as the engine names them. */
    var tiles by mutableStateOf(sp.getString(TILES, "") ?: "")
        private set
    var haptics by mutableStateOf(sp.getBoolean(HAPTICS, true))
        private set

    fun chooseTheme(v: String) {
        theme = v
        sp.edit().putString(THEME, v).apply()
    }

    fun chooseAppearance(v: Appearance) {
        appearance = v
        sp.edit().putString(APPEARANCE, v.name).apply()
    }

    fun chooseTiles(v: String) {
        tiles = v
        sp.edit().putString(TILES, v).apply()
    }

    fun chooseHaptics(v: Boolean) {
        haptics = v
        sp.edit().putBoolean(HAPTICS, v).apply()
    }

    companion object {
        private const val THEME = "theme"
        private const val APPEARANCE = "appearance"
        private const val TILES = "tiles"
        private const val HAPTICS = "haptics"

        fun of(context: Context) = Prefs(context.getSharedPreferences("surmise", Context.MODE_PRIVATE))
    }
}
