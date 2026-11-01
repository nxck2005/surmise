package com.nxck.surmise

import android.content.Context
import com.nxck.surmise.core.CoreHandle
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock

/**
 * The one engine of the process. An activity comes and goes (the back
 * gesture at home finishes it, the process stays), but two engines over one
 * data dir would each think they own the store, so every activity shares
 * this one. It is never closed: every guess is saved as it lands, and
 * leaving the board on stop banks the clock, so a killed process loses
 * nothing.
 */
object Engine {
    private val lock = Mutex()
    @Volatile private var handle: CoreHandle? = null

    suspend fun get(context: Context): CoreHandle {
        handle?.let { return it }
        return lock.withLock {
            handle ?: CoreHandle.open(context.applicationContext).also { handle = it }
        }
    }
}
