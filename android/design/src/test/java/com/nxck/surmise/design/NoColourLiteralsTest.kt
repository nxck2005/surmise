package com.nxck.surmise.design

import java.io.File
import org.junit.Assert.assertTrue
import org.junit.Test

// Every colour on screen comes from a theme's tokens, so a new theme can
// restyle the whole app. A literal colour in a component would ignore the
// theme, so this test fails the build when one appears in :app or :design.
// Tokens.kt (the shadow's black) and Icons.kt (the black that icons are
// tinted from) are the only files allowed to name one.
class NoColourLiteralsTest {
    private val allowed = setOf("Tokens.kt", "Icons.kt")

    private val banned = listOf(
        Regex("""Color\(\s*0x"""),
        Regex("""Color\(\s*red\s*="""),
        Regex("""Color\(\s*[0-9]"""),
        // Color.Transparent and Color.Unspecified carry no hue, so they stay.
        Regex("""Color\.(?!Transparent\b|Unspecified\b)[A-Z][A-Za-z]*"""),
        Regex(""""#[0-9a-fA-F]{6}([0-9a-fA-F]{2})?""""),
    )

    @Test
    fun noColourLiterals() {
        val roots = listOf(File("src/main/java"), File("../app/src/main/java"))
        val files = roots.flatMap { root -> root.walk().filter { it.extension == "kt" }.toList() }
        assertTrue("no sources found; was the working directory moved?", files.size > 5)
        val hits = files
            .filter { it.name !in allowed }
            .flatMap { file ->
                file.readLines().mapIndexedNotNull { i, line ->
                    val code = line.substringBefore("//")
                    if (banned.any { it.containsMatchIn(code) }) "${file.path}:${i + 1}: ${line.trim()}" else null
                }
            }
        assertTrue("colour literals outside the tokens:\n" + hits.joinToString("\n"), hits.isEmpty())
    }
}
