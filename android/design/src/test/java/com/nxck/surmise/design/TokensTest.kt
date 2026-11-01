package com.nxck.surmise.design

import java.io.File
import kotlinx.serialization.SerializationException
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

// The bundled JSON is what genandroid writes from internal/theme; these tests
// read the same files the app ships, so a contract change on the Go side
// fails here before it fails on a phone.
class TokensTest {
    private fun bundled(name: String) = File("src/main/assets/tokens/$name.json").readText()

    @Test
    fun nightDecodes() {
        val t = SurmiseTokens.fromJson(bundled("night"))
        assertEquals("night", t.name)
        assertEquals("day", t.pair)
        assertTrue(t.dark)
        assertEquals(hex("#000000"), t.canvas)
        assertEquals(hex("#ff8a3d"), t.action)
        assertEquals(hex("#2f9e5e"), t.tiles.correct.fill)
    }

    @Test
    fun dayDecodes() {
        val t = SurmiseTokens.fromJson(bundled("day"))
        assertEquals("day", t.name)
        assertEquals("night", t.pair)
        assertTrue(!t.dark)
        assertEquals(hex("#f2f2f7"), t.canvas)
    }

    @Test
    fun pairIsOptional() {
        val text = bundled("night").replace("\"pair\": \"day\",", "")
        assertNull(SurmiseTokens.fromJson(text).pair)
    }

    @Test
    fun unknownFieldFails() {
        // A field the engine adds must be added here too, not dropped.
        val text = bundled("night").replaceFirst("{", "{\n  \"glowMid\": \"#123456\",")
        assertThrows(SerializationException::class.java) { SurmiseTokens.fromJson(text) }
    }

    @Test
    fun badHexFails() {
        for (bad in listOf("", "#fff", "000000", "#00000000", "#gggggg")) {
            assertThrows(IllegalArgumentException::class.java) { hex(bad) }
        }
    }
}
