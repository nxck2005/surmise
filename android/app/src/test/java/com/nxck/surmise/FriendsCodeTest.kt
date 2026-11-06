package com.nxck.surmise

import com.nxck.surmise.friends.CodeKind
import com.nxck.surmise.friends.codeKind
import com.nxck.surmise.friends.normalizeCode
import org.junit.Assert.assertEquals
import org.junit.Test

// One field takes both kinds of code; its length once tidied says which.
class FriendsCodeTest {
    @Test
    fun spacesAndDashesGoAndLettersGoUp() {
        assertEquals("KVR7QD", normalizeCode(" kvr 7qd "))
        assertEquals("7K2MQX4R9TPWB3HD", normalizeCode("7k2m-qx4r-9tpw-b3hd"))
    }

    @Test
    fun sixIsARoomAndSixteenIsAChallenge() {
        assertEquals(CodeKind.Room, codeKind(normalizeCode("kvr-7qd")))
        assertEquals(CodeKind.Challenge, codeKind(normalizeCode("7K2M-QX4R-9TPW-B3HD")))
        assertEquals(null, codeKind(normalizeCode("KVR7Q")))
        assertEquals(null, codeKind(""))
    }
}
