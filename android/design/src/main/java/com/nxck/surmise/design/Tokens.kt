package com.nxck.surmise.design

import android.content.Context
import android.content.res.AssetManager
import androidx.compose.runtime.Immutable
import androidx.compose.ui.graphics.Color
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json

// The app's colours come from one place: the engine resolves a theme file into
// theme.AppTokens (internal/theme/app.go), and this file turns that JSON into
// Compose colours. Nothing in :app or :design picks a colour of its own; a
// test (NoColourLiteralsTest) fails the build when one does. This file is one
// of the two allowed to name a colour, because it holds the few that are not
// a theme's to choose: the black of a shadow and a scrim.

/** One mark's fill and the letter drawn on it. */
@Immutable
data class TileColors(val fill: Color, val letter: Color)

/** The three marks. */
@Immutable
data class TilePalette(val correct: TileColors, val present: TileColors, val absent: TileColors)

/**
 * Every colour the app draws with, field for field the Go side's AppTokens.
 * A theme swap replaces the whole object.
 */
@Immutable
data class SurmiseTokens(
    val name: String,
    val pair: String?,
    val fonts: String,
    val dark: Boolean,

    val canvas: Color,
    val card: Color,
    val sheet: Color,
    val highlight: Color,

    val ink: Color,
    val inkDim: Color,
    val inkMuted: Color,

    val accent: Color,
    val accentText: Color,
    val action: Color,
    val onAction: Color,
    val danger: Color,

    val chartHighlight: Color,
    val chartNeutral: Color,

    val keyFace: Color,
    val keySpecial: Color,
    val keySpent: Color,
    val keyText: Color,
    val keySpentText: Color,

    val tileEmpty: Color,
    val tileTyped: Color,
    val cursor: Color,

    val tiles: TilePalette,

    val glowFrom: Color,
    val glowTo: Color,
    val tabTint: Color,
) {
    /** Shadows are always black, at an alpha the caller picks. */
    val shadow: Color get() = Color.Black

    /** The dimming behind a sheet: darker on a dark canvas, where black barely shows. */
    val scrim: Color get() = Color.Black.copy(alpha = if (dark) 0.55f else 0.30f)

    companion object {
        private val json = Json {
            // A field the engine adds that this side does not know is a bug to
            // hear about, not a value to drop.
            ignoreUnknownKeys = false
        }

        /** Decodes the engine's themes.tokens JSON (theme.AppTokens). */
        fun fromJson(text: String): SurmiseTokens = json.decodeFromString(TokensDto.serializer(), text).toTokens()
    }
}

/** Tokens bundled in the module's assets, so previews and tests need no engine. */
object BundledTokens {
    // Written by `go run ./tools/genandroid` from the bundled night and day
    // themes; CI fails when they are stale. Do not edit the JSON by hand.
    private const val DIR = "tokens"

    @Volatile private var cache: Map<String, SurmiseTokens> = emptyMap()

    fun night(context: Context): SurmiseTokens = load(context.assets, "night")

    fun day(context: Context): SurmiseTokens = load(context.assets, "day")

    /** The bundled pair for the system appearance. */
    fun forAppearance(context: Context, dark: Boolean): SurmiseTokens = if (dark) night(context) else day(context)

    fun load(assets: AssetManager, name: String): SurmiseTokens {
        cache[name]?.let { return it }
        val text = assets.open("$DIR/$name.json").bufferedReader().use { it.readText() }
        return SurmiseTokens.fromJson(text).also { cache = cache + (name to it) }
    }
}

@Serializable
private data class TileDto(val fill: String, val letter: String)

@Serializable
private data class TilesDto(val correct: TileDto, val present: TileDto, val absent: TileDto)

@Serializable
private data class TokensDto(
    val name: String,
    val pair: String? = null,
    val fonts: String,
    val dark: Boolean,
    val canvas: String,
    val card: String,
    val sheet: String,
    val highlight: String,
    val ink: String,
    val inkDim: String,
    val inkMuted: String,
    val accent: String,
    val accentText: String,
    val action: String,
    val onAction: String,
    val danger: String,
    val chartHighlight: String,
    val chartNeutral: String,
    val keyFace: String,
    val keySpecial: String,
    val keySpent: String,
    val keyText: String,
    val keySpentText: String,
    val tileEmpty: String,
    val tileTyped: String,
    val cursor: String,
    val tiles: TilesDto,
    val glowFrom: String,
    val glowTo: String,
    val tabTint: String,
) {
    fun toTokens() = SurmiseTokens(
        name = name,
        pair = pair,
        fonts = fonts,
        dark = dark,
        canvas = hex(canvas),
        card = hex(card),
        sheet = hex(sheet),
        highlight = hex(highlight),
        ink = hex(ink),
        inkDim = hex(inkDim),
        inkMuted = hex(inkMuted),
        accent = hex(accent),
        accentText = hex(accentText),
        action = hex(action),
        onAction = hex(onAction),
        danger = hex(danger),
        chartHighlight = hex(chartHighlight),
        chartNeutral = hex(chartNeutral),
        keyFace = hex(keyFace),
        keySpecial = hex(keySpecial),
        keySpent = hex(keySpent),
        keyText = hex(keyText),
        keySpentText = hex(keySpentText),
        tileEmpty = hex(tileEmpty),
        tileTyped = hex(tileTyped),
        cursor = hex(cursor),
        tiles = TilePalette(tiles.correct.toColors(), tiles.present.toColors(), tiles.absent.toColors()),
        glowFrom = hex(glowFrom),
        glowTo = hex(glowTo),
        tabTint = hex(tabTint),
    )
}

private fun TileDto.toColors() = TileColors(hex(fill), hex(letter))

/** Parses the engine's `#rrggbb`. Anything else is a contract break, so it throws. */
fun hex(s: String): Color {
    require(s.length == 7 && s[0] == '#') { "want #rrggbb, got \"$s\"" }
    val rgb = s.substring(1).toLong(16)
    return Color(0xFF000000 or rgb)
}
