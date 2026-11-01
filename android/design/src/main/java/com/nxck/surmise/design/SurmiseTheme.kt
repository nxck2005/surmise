package com.nxck.surmise.design

import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp

// A placeholder for the design system. P4b replaces Tokens with SurmiseTokens,
// decoded from the engine's theme JSON, and adds the fonts, shapes and shared
// components. These few values are the night theme's, so the skeleton already
// looks like the app; they are the only colour literals allowed outside the
// generated tokens, and they go when P4b lands.

@Immutable
data class Tokens(
    val canvas: Color,
    val ink: Color,
    val inkMuted: Color,
)

@Immutable
data class Type(
    val title: TextStyle,
    val caption: TextStyle,
)

private val night = Tokens(
    canvas = Color(0xFF000000),
    ink = Color(0xFFF2F4F8),
    inkMuted = Color(0xFF7C8496),
)

private fun typeFor(t: Tokens) = Type(
    title = TextStyle(color = t.ink, fontSize = 28.sp, fontWeight = FontWeight.Medium),
    caption = TextStyle(color = t.inkMuted, fontSize = 15.sp),
)

private val LocalTokens = staticCompositionLocalOf { night }
private val LocalType = staticCompositionLocalOf { typeFor(night) }

@Composable
fun SurmiseTheme(content: @Composable () -> Unit) {
    CompositionLocalProvider(
        LocalTokens provides night,
        LocalType provides typeFor(night),
        content = content,
    )
}

object SurmiseTheme {
    val tokens: Tokens
        @Composable get() = LocalTokens.current

    val type: Type
        @Composable get() = LocalType.current
}
