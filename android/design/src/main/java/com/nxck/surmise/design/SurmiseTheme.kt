package com.nxck.surmise.design

import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.text.BasicText
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.State
import androidx.compose.runtime.remember
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.style.TextAlign

/**
 * How much the app moves. Off is the system's "remove animations" or the
 * player's choice. [calm] is the "restrained" setting: the board's own
 * effects (the flip, its stagger, the hops and the shake) take half the time
 * and travel half as far, as in the terminal. The chrome moves the same.
 */
@Immutable
data class Motion(val enabled: Boolean = true, val calm: Boolean = false) {
    /** A board effect's duration at this level. */
    fun ms(full: Int): Int = if (calm) full / 2 else full

    /** A board effect's travel at this level. */
    fun reach(full: Float): Float = if (calm) full / 2 else full
}

private val LocalTokens = staticCompositionLocalOf<SurmiseTokens> { error("no SurmiseTheme above this composable") }
private val LocalType = staticCompositionLocalOf { DefaultType }
private val LocalMotion = staticCompositionLocalOf { Motion() }
internal val LocalHapticsOn = staticCompositionLocalOf { true }

/** How long a theme change cross-fades. */
const val THEME_FADE_MS = 250

/**
 * The root of every screen. [tokens] come from the engine (or [BundledTokens]
 * in previews and tests); swapping them cross-fades every colour, so the
 * theme picker's live preview needs nothing of its own.
 */
@Composable
fun SurmiseTheme(
    tokens: SurmiseTokens,
    motion: Motion = Motion(),
    haptics: Boolean = true,
    content: @Composable () -> Unit,
) {
    val shown = if (motion.enabled) animateTokens(tokens) else tokens
    CompositionLocalProvider(
        LocalTokens provides shown,
        LocalType provides DefaultType,
        LocalMotion provides motion,
        LocalHapticsOn provides haptics,
        content = content,
    )
}

/** [SurmiseTheme] over the bundled night or day tokens, by the system appearance. */
@Composable
fun SurmiseTheme(dark: Boolean = isSystemInDarkTheme(), content: @Composable () -> Unit) {
    val context = LocalContext.current
    val tokens = remember(dark) { BundledTokens.forAppearance(context, dark) }
    SurmiseTheme(tokens = tokens, content = content)
}

object SurmiseTheme {
    val tokens: SurmiseTokens
        @Composable get() = LocalTokens.current

    val type: SurmiseType
        @Composable get() = LocalType.current

    val motion: Motion
        @Composable get() = LocalMotion.current
}

/** Text in a type style and a token colour. */
@Composable
fun SurmiseText(
    text: String,
    style: TextStyle,
    color: Color,
    modifier: Modifier = Modifier,
    align: TextAlign? = null,
    maxLines: Int = Int.MAX_VALUE,
) {
    BasicText(
        text = text,
        modifier = modifier,
        style = if (align != null) style.copy(color = color, textAlign = align) else style.copy(color = color),
        maxLines = maxLines,
    )
}

@Composable
private fun fade(target: Color): State<Color> = animateColorAsState(target, tween(THEME_FADE_MS), label = "theme")

@Composable
private fun fade(target: TileColors) = TileColors(fade(target.fill).value, fade(target.letter).value)

@Composable
private fun animateTokens(t: SurmiseTokens): SurmiseTokens = t.copy(
    canvas = fade(t.canvas).value,
    card = fade(t.card).value,
    sheet = fade(t.sheet).value,
    highlight = fade(t.highlight).value,
    ink = fade(t.ink).value,
    inkDim = fade(t.inkDim).value,
    inkMuted = fade(t.inkMuted).value,
    accent = fade(t.accent).value,
    accentText = fade(t.accentText).value,
    action = fade(t.action).value,
    onAction = fade(t.onAction).value,
    danger = fade(t.danger).value,
    chartHighlight = fade(t.chartHighlight).value,
    chartNeutral = fade(t.chartNeutral).value,
    keyFace = fade(t.keyFace).value,
    keySpecial = fade(t.keySpecial).value,
    keySpent = fade(t.keySpent).value,
    keyText = fade(t.keyText).value,
    keySpentText = fade(t.keySpentText).value,
    tileEmpty = fade(t.tileEmpty).value,
    tileTyped = fade(t.tileTyped).value,
    cursor = fade(t.cursor).value,
    tiles = TilePalette(fade(t.tiles.correct), fade(t.tiles.present), fade(t.tiles.absent)),
    glowFrom = fade(t.glowFrom).value,
    glowTo = fade(t.glowTo).value,
    tabTint = fade(t.tabTint).value,
)
