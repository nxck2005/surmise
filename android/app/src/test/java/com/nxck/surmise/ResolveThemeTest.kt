package com.nxck.surmise

import com.nxck.surmise.core.gen.ThemeInfo
import com.nxck.surmise.settings.Appearance
import org.junit.Assert.assertEquals
import org.junit.Test

class ResolveThemeTest {
    private val themes = listOf(
        ThemeInfo(name = "dusk", dark = true, pair = "dawn"),
        ThemeInfo(name = "dawn", dark = false, pair = "dusk"),
        ThemeInfo(name = "neon", dark = true),
    )

    @Test
    fun bundledFollowsAppearance() {
        assertEquals("night", resolveTheme("", Appearance.System, true, themes))
        assertEquals("day", resolveTheme("", Appearance.System, false, themes))
        assertEquals("night", resolveTheme("", Appearance.Dark, false, themes))
        assertEquals("day", resolveTheme("", Appearance.Light, true, themes))
    }

    @Test
    fun pairSwapsToTheWantedSide() {
        assertEquals("dawn", resolveTheme("dusk", Appearance.Light, true, themes))
        assertEquals("dusk", resolveTheme("dawn", Appearance.System, true, themes))
        assertEquals("dusk", resolveTheme("dusk", Appearance.Dark, false, themes))
    }

    @Test
    fun aThemeWithNoPairStays() {
        assertEquals("neon", resolveTheme("neon", Appearance.Light, false, themes))
        assertEquals("gone", resolveTheme("gone", Appearance.Light, false, themes))
    }
}
