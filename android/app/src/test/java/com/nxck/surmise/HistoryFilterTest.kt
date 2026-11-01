package com.nxck.surmise

import com.nxck.surmise.history.HistoryFilter
import com.nxck.surmise.history.shortDate
import org.junit.Assert.assertEquals
import org.junit.Test

// The history's filters are the TUI's three fields: length, kind, status.
class HistoryFilterTest {
    private fun ids(f: HistoryFilter) = historyItems.filter(f::matches).map { it.id }

    @Test
    fun anyLetsEverythingThrough() = assertEquals(historyItems.map { it.id }, ids(HistoryFilter()))

    @Test
    fun theFieldsCombine() {
        assertEquals(listOf("d-4", "r-4-9"), ids(HistoryFilter(length = 4)))
        assertEquals(listOf("r-4-9"), ids(HistoryFilter(length = 4, kind = "random")))
        assertEquals(listOf("d-4", "r-4-9"), ids(HistoryFilter(status = "won")))
        assertEquals(emptyList<String>(), ids(HistoryFilter(length = 6, status = "won")))
    }

    @Test
    fun aBadTimeIsBlankNotACrash() = assertEquals("", shortDate("yesterday"))
}
