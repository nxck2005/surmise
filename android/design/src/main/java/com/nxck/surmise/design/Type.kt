package com.nxck.surmise.design

import androidx.compose.runtime.Immutable
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.em
import androidx.compose.ui.unit.sp

// Geist for the interface, Geist Mono for codes and clocks, Geist Serif for
// the few display lines (the wordmark, a result's headline). All three are
// the Geist Project's, under the SIL Open Font License 1.1; the licence text
// ships in assets/licenses/geist-OFL.txt for the about screen.
//
// Sizes are the mockups'. They are not themeable: a theme changes colours,
// never sizes, so it can never move anything.

val Geist = FontFamily(
    Font(R.font.geist_regular, FontWeight.Normal),
    Font(R.font.geist_medium, FontWeight.Medium),
    Font(R.font.geist_semibold, FontWeight.SemiBold),
)

val GeistMono = FontFamily(
    Font(R.font.geist_mono_regular, FontWeight.Normal),
    Font(R.font.geist_mono_medium, FontWeight.Medium),
)

val GeistSerif = FontFamily(
    Font(R.font.geist_serif_regular, FontWeight.Normal),
)

/** The type scale. Styles carry no colour; the caller picks a token. */
@Immutable
data class SurmiseType(
    /** The wordmark on home: serif 38/42. */
    val display: TextStyle,
    /** A result or done line: serif 22/28. */
    val displaySmall: TextStyle,
    /** A card's big line ("today's three", "5 letters"): 26/32 semibold. */
    val title: TextStyle,
    /** A screen's name in its top bar: 16/20 medium. */
    val headline: TextStyle,
    /** A section heading ("play"): 15/20 semibold. */
    val section: TextStyle,
    /** Body text and button labels: 14/20 medium. */
    val body: TextStyle,
    /** Small labels and notes: 13/16 medium. */
    val caption: TextStyle,
    /** The quietest line: 12/16. */
    val footnote: TextStyle,
    /** Tab bar labels: 10/12 semibold. */
    val tab: TextStyle,
    /** Puzzle codes: mono 12/16. */
    val code: TextStyle,
    /** The board's clock: mono 14/20, tabular. */
    val clock: TextStyle,
    /** A letter on a board tile: 28 semibold. */
    val tile: TextStyle,
    /** A letter on a key: 18 medium. */
    val key: TextStyle,
    /** "enter" on its key: 13 medium. */
    val keySpecial: TextStyle,
)

private const val TABULAR = "tnum"

val DefaultType = SurmiseType(
    display = TextStyle(fontFamily = GeistSerif, fontSize = 38.sp, lineHeight = 42.sp, letterSpacing = (-0.02).em),
    displaySmall = TextStyle(fontFamily = GeistSerif, fontSize = 22.sp, lineHeight = 28.sp),
    title = TextStyle(fontFamily = Geist, fontWeight = FontWeight.SemiBold, fontSize = 26.sp, lineHeight = 32.sp, letterSpacing = (-0.02).em),
    headline = TextStyle(fontFamily = Geist, fontWeight = FontWeight.Medium, fontSize = 16.sp, lineHeight = 20.sp),
    section = TextStyle(fontFamily = Geist, fontWeight = FontWeight.SemiBold, fontSize = 15.sp, lineHeight = 20.sp),
    body = TextStyle(fontFamily = Geist, fontWeight = FontWeight.Medium, fontSize = 14.sp, lineHeight = 20.sp),
    caption = TextStyle(fontFamily = Geist, fontWeight = FontWeight.Medium, fontSize = 13.sp, lineHeight = 16.sp, fontFeatureSettings = TABULAR),
    footnote = TextStyle(fontFamily = Geist, fontSize = 12.sp, lineHeight = 16.sp, fontFeatureSettings = TABULAR),
    tab = TextStyle(fontFamily = Geist, fontWeight = FontWeight.SemiBold, fontSize = 10.sp, lineHeight = 12.sp),
    code = TextStyle(fontFamily = GeistMono, fontSize = 12.sp, lineHeight = 16.sp),
    clock = TextStyle(fontFamily = GeistMono, fontSize = 14.sp, lineHeight = 20.sp, fontFeatureSettings = TABULAR),
    tile = TextStyle(fontFamily = Geist, fontWeight = FontWeight.SemiBold, fontSize = 28.sp, lineHeight = 28.sp),
    key = TextStyle(fontFamily = Geist, fontWeight = FontWeight.Medium, fontSize = 18.sp, lineHeight = 22.sp),
    keySpecial = TextStyle(fontFamily = Geist, fontWeight = FontWeight.Medium, fontSize = 13.sp, lineHeight = 16.sp),
)
