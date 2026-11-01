package com.nxck.surmise.core

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.os.Build
import com.nxck.surmise.core.gen.CoreClient
import com.nxck.surmise.core.gen.ImportReport
import com.nxck.surmise.core.gen.decodeEnvelope
import com.nxck.surmise.engine.core.Core
import com.nxck.surmise.engine.core.Engine
import com.nxck.surmise.engine.core.Listener
import java.io.File
import java.util.TimeZone
import java.util.concurrent.Executors
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.ExecutorCoroutineDispatcher
import kotlinx.coroutines.asCoroutineDispatcher
import kotlinx.coroutines.channels.BufferOverflow
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.withContext

/** One event from the engine, sent on a Go thread. payload is JSON. */
data class Event(val kind: String, val payload: String)

/**
 * The open Go engine. Every call runs on one background thread, never the
 * main one: the engine serialises calls anyway, and one thread keeps the
 * order the app made them in.
 */
class CoreHandle private constructor(
    private val engine: Engine,
    private val dispatcher: ExecutorCoroutineDispatcher,
    private val context: Context,
    private val bus: MutableSharedFlow<Event>,
) {
    /** The typed methods (generated from the Go registry). */
    val client = CoreClient { method, json ->
        withContext(dispatcher) {
            String(engine.call(method, json.toByteArray(Charsets.UTF_8)), Charsets.UTF_8)
        }
    }

    /** Events from Go threads: room messages, from Phase 4 on. */
    val events: SharedFlow<Event> = bus.asSharedFlow()

    private val zoneReceiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context, intent: Intent) {
            val zone = TimeZone.getDefault().id
            dispatcher.executor.execute { runCatching { engine.setZone(zone) } }
        }
    }

    init {
        val filter = IntentFilter(Intent.ACTION_TIMEZONE_CHANGED)
        if (Build.VERSION.SDK_INT >= 33) {
            context.registerReceiver(zoneReceiver, filter, Context.RECEIVER_NOT_EXPORTED)
        } else {
            context.registerReceiver(zoneReceiver, filter)
        }
    }

    /** The whole install as a backup archive, for the file picker to save. */
    suspend fun exportBackup(): ByteArray = withContext(dispatcher) { engine.exportBackup() }

    /** Merges an archive into the install. Throws CoreException on a bad file. */
    suspend fun importBackup(bytes: ByteArray): ImportReport = withContext(dispatcher) {
        decodeEnvelope(String(engine.importBackup(bytes), Charsets.UTF_8), ImportReport.serializer())
    }

    /** Banks and saves the open board, then lets go of the engine's thread. */
    suspend fun close() {
        runCatching { context.unregisterReceiver(zoneReceiver) }
        withContext(dispatcher) { engine.close() }
        dispatcher.close()
    }

    companion object {
        /** The data dir, the native store layout under the app's own files. */
        fun dataDir(context: Context): File = File(context.filesDir, "surmise")

        suspend fun open(context: Context): CoreHandle {
            val app = context.applicationContext
            val dispatcher = Executors.newSingleThreadExecutor { r ->
                Thread(r, "surmise-engine").apply { isDaemon = true }
            }.asCoroutineDispatcher()
            val bus = MutableSharedFlow<Event>(
                extraBufferCapacity = 64,
                onBufferOverflow = BufferOverflow.DROP_OLDEST,
            )
            val listener = object : Listener {
                override fun onEvent(kind: String, payload: ByteArray) {
                    bus.tryEmit(Event(kind, String(payload, Charsets.UTF_8)))
                }
            }
            val engine = withContext(dispatcher as CoroutineDispatcher) {
                val dir = dataDir(app).apply { mkdirs() }
                val version = app.packageManager.getPackageInfo(app.packageName, 0).versionName ?: "dev"
                Core.open(dir.path, TimeZone.getDefault().id, version, Build.SUPPORTED_ABIS.firstOrNull() ?: "", listener)
            }
            return CoreHandle(engine, dispatcher, app, bus)
        }
    }
}
