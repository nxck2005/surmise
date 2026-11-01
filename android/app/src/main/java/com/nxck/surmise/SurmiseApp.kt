package com.nxck.surmise

import android.app.Activity
import android.content.Context
import android.content.Intent
import android.provider.Settings as SystemSettings
import androidx.activity.compose.BackHandler
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.background
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.SideEffect
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalView
import androidx.core.view.WindowCompat
import com.nxck.surmise.core.gen.SettingsPatch
import com.nxck.surmise.settings.AboutScreen
import com.nxck.surmise.settings.HowToScreen
import com.nxck.surmise.settings.SettingsActions
import com.nxck.surmise.settings.SettingsScreen
import com.nxck.surmise.settings.SettingsState
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import com.nxck.surmise.board.BoardScreen
import com.nxck.surmise.daily.DailyScreen
import com.nxck.surmise.history.HistoryScreen
import com.nxck.surmise.design.BundledTokens
import com.nxck.surmise.design.Motion
import com.nxck.surmise.design.SettleEasing
import com.nxck.surmise.design.SurmiseIcons
import com.nxck.surmise.design.SurmiseText
import com.nxck.surmise.design.SurmiseTheme
import com.nxck.surmise.design.TabBar
import com.nxck.surmise.design.TabItem
import com.nxck.surmise.design.glassSource
import com.nxck.surmise.design.rememberGlassState
import com.nxck.surmise.home.HomeActions
import com.nxck.surmise.home.HomeScreen
import com.nxck.surmise.home.homeState
import com.nxck.surmise.modes.ChallengeScreen
import com.nxck.surmise.modes.CustomScreen
import com.nxck.surmise.sprint.SprintClock
import com.nxck.surmise.rooms.RoomActions
import com.nxck.surmise.rooms.RoomLines
import com.nxck.surmise.rooms.RoomResultScreen
import com.nxck.surmise.rooms.RoomScreen
import com.nxck.surmise.rooms.RoomsScreen
import com.nxck.surmise.rooms.RoundClock
import com.nxck.surmise.sprint.SprintModel
import com.nxck.surmise.sprint.SprintSetup
import com.nxck.surmise.sprint.SprintSummary
import com.nxck.surmise.sprint.sprintTally
import com.nxck.surmise.record.RecordScreen
import com.nxck.surmise.record.YearScreen

/** How long a pushed screen takes to slide in or out. */
private const val PUSH_MS = 320

/** The app: the theme, the tabs, and the screens pushed over them. */
@Composable
fun SurmiseApp(model: AppModel) {
    val context = LocalContext.current
    val name = model.themeName(isSystemInDarkTheme())
    val tiles = model.prefs.tiles
    val bundled = name in setOf("night", "day") && tiles.isEmpty()
    LaunchedEffect(name, tiles) { if (!bundled) model.loadTokens(name) }
    // Until a theme's tokens arrive, the bundled side it is closest to.
    val tokens = if (bundled) {
        BundledTokens.load(context.assets, name)
    } else {
        model.themeTokens["$name/$tiles"] ?: BundledTokens.forAppearance(context, model.themes.firstOrNull { it.name == name }?.dark ?: true)
    }
    val motion = Motion(enabled = model.settings.motion != "off" && systemAnimates(context), calm = model.settings.motion == "restrained")
    val view = LocalView.current
    SideEffect {
        (view.context as? Activity)?.window?.let { w ->
            WindowCompat.getInsetsController(w, view).apply {
                isAppearanceLightStatusBars = !tokens.dark
                isAppearanceLightNavigationBars = !tokens.dark
            }
        }
    }

    SurmiseTheme(tokens = tokens, motion = motion, haptics = model.prefs.haptics) {
        LaunchedEffect(Unit) {
            model.refreshHome()
            model.refreshSettings()
            model.start()
        }
        BackHandler(enabled = model.handlesBack) { model.back() }
        AnimatedContent(
            targetState = model.top,
            transitionSpec = {
                val pushing = targetState.depth > initialState.depth
                val spec = tween<androidx.compose.ui.unit.IntOffset>(if (motion.enabled) PUSH_MS else 0, easing = SettleEasing)
                val fade = tween<Float>(if (motion.enabled) PUSH_MS else 0)
                if (pushing) {
                    (slideInHorizontally(spec) { it } + fadeIn(fade)) togetherWith (slideOutHorizontally(spec) { -it / 4 } + fadeOut(fade))
                } else {
                    (slideInHorizontally(spec) { -it / 4 } + fadeIn(fade)) togetherWith (slideOutHorizontally(spec) { it } + fadeOut(fade))
                }
            },
            modifier = Modifier.fillMaxSize().background(SurmiseTheme.tokens.canvas),
            label = "route",
        ) { route ->
            when (route) {
                Route.Tabs -> Tabs(model)
                Route.Daily -> DailyScreen(
                    daily = model.daily,
                    date = homeDate(),
                    playedToday = if (model.online.consent) model.online.menuCount else 0,
                    onBack = model::back,
                    onOpen = model::openDaily,
                    onShare = { share(context, it) },
                    counts = if (model.online.consent) model.dailyCounts else emptyMap(),
                )
                Route.Year -> YearScreen(
                    calendar = model.calendar,
                    trends = model.trends,
                    onBack = model::back,
                    startLength = model.settings.length.takeIf { it in 4..6 } ?: 5,
                )
                Route.HowTo -> HowToScreen(onBack = model::back)
                Route.About -> AboutScreen(model.info, onBack = model::back)
                Route.Sprint -> SprintSetup(model.sprintOptions, model.startLength, onStart = model::startSprint, onBack = model::back)
                Route.Challenge -> ChallengeScreen(
                    state = model.challenge,
                    startLength = model.startLength,
                    onMake = model::makeChallenge,
                    onShare = { share(context, it) },
                    onOpen = model::openChallenge,
                    onBack = model::back,
                )
                Route.Custom -> CustomScreen(
                    startLength = model.startLength,
                    note = model.customNote,
                    onCheck = model::checkCustom,
                    onReady = model::startCustom,
                    onBack = model::back,
                )
                Route.Rooms -> RoomsScreen(
                    list = model.rooms.list,
                    lines = RoomLines(model.rooms.error, model.rooms.note),
                    startLength = model.startLength,
                    onMake = model::makeRoom,
                    onJoin = model::joinRoom,
                    onLeave = model.rooms::leave,
                    onBack = model::back,
                )
                Route.Room -> Room(model)
                Route.RoomResult -> model.roomResult?.let { r ->
                    RoomResultScreen(
                        r,
                        RoomLines(model.roomResultError, model.rooms.note),
                        RoomActions(onRefresh = model::refreshRoomResult, onAway = model::back, onReview = model::reviewRoomResult),
                    )
                }
                Route.Board -> when (val b = model.board) {
                    is SprintModel -> BoardScreen(
                        b,
                        onBack = { model.closeBoard() },
                        onHome = model::goHome,
                        onShare = { share(context, it) },
                        title = stringResource(R.string.sprint),
                        subtitle = sprintTally(b.run),
                        trailing = { SprintClock(b) },
                        sheet = { SprintSummary(b, onAgain = { model.startSprint(b.run.length, b.run.durationMs) }, onHome = model::goHome) },
                    )
                    null -> Unit
                    else -> BoardScreen(b, onBack = { model.closeBoard() }, onHome = model::goHome, onShare = { share(context, it) })
                }
            }
        }
    }
}

/** The open room: our board while our round is on, else the room's own screen. */
@Composable
private fun Room(model: AppModel) {
    val context = LocalContext.current
    val rooms = model.rooms
    val room = rooms.room
    val b = rooms.board
    if (room.screen == "board" && b != null) {
        val solved = room.players.count { it.solved }
        BoardScreen(
            b,
            onBack = model::awayFromRoom,
            onHome = {
                rooms.close()
                model.goHome()
            },
            onShare = { share(context, it) },
            title = stringResource(R.string.room_title, room.code),
            subtitle = if (room.players.size > 1) stringResource(R.string.room_solved_of, solved, room.players.size) else null,
            trailing = if (room.mode == "live") ({ RoundClock(room.endsAtMs) }) else null,
        )
        return
    }
    RoomScreen(
        room,
        RoomLines(rooms.error, rooms.note),
        RoomActions(
            onReady = rooms::ready,
            onStart = rooms::start,
            onRematch = rooms::rematch,
            onRefresh = rooms::refreshResult,
            onShare = { share(context, it) },
            onLeave = model::leaveRoom,
            onAway = model::awayFromRoom,
        ),
    )
}

@Composable
private fun Tabs(model: AppModel) {
    val tokens = SurmiseTheme.tokens
    val glass = rememberGlassState()
    val items = listOf(
        TabItem(stringResource(R.string.tab_play), SurmiseIcons.Play),
        TabItem(stringResource(R.string.tab_history), SurmiseIcons.History),
        TabItem(stringResource(R.string.tab_record), SurmiseIcons.Record),
        TabItem(stringResource(R.string.tab_settings), SurmiseIcons.Settings),
    )
    Box(Modifier.fillMaxSize().background(tokens.canvas)) {
        Box(Modifier.fillMaxSize().glassSource(glass)) {
            when (model.tab) {
                Tab.Play -> HomeScreen(
                    state = homeState(
                        name = model.settings.displayName,
                        daily = model.daily,
                        history = model.history,
                        playedToday = if (model.online.consent) model.online.menuCount else 0,
                        note = model.homeNote,
                        rooms = model.online.rooms && model.online.consent,
                        running = model.rooms.room.takeIf { it.active && it.probe }?.code.orEmpty(),
                    ),
                    actions = HomeActions(
                        onDaily = model::openDaily,
                        onDailyScreen = model::showDaily,
                        onPlay = model::playLength,
                        onRecord = { model.select(Tab.Record) },
                        onWay = model::showWay,
                        onRoom = model::showRoom,
                    ),
                )
                Tab.History -> HistoryScreen(
                    items = model.history,
                    onOpen = model::openSaved,
                    onDelete = { model.delete(it.id) },
                )
                Tab.Record -> RecordScreen(model.profile, model.calendar, onYear = model::showYear)
                Tab.Settings -> Settings(model)
            }
        }
        TabBar(
            items = items,
            selected = model.tab.ordinal,
            onSelect = { model.select(Tab.entries[it]) },
            glass = glass,
            modifier = Modifier
                .align(Alignment.BottomCenter)
                .navigationBarsPadding()
                .padding(start = 20.dp, end = 20.dp, bottom = 12.dp),
        )
    }
}

@Composable
private fun Settings(model: AppModel) {
    val context = LocalContext.current
    val save = rememberLauncherForActivityResult(ActivityResultContracts.CreateDocument("application/json")) { uri ->
        if (uri != null) {
            model.exportBackup { bytes ->
                context.contentResolver.openOutputStream(uri)?.use { it.write(bytes) } ?: error("no stream")
            }
        }
    }
    val open = rememberLauncherForActivityResult(ActivityResultContracts.OpenDocument()) { uri ->
        if (uri != null) {
            model.importBackup {
                context.contentResolver.openInputStream(uri)?.use { it.readBytes() } ?: error("no stream")
            }
        }
    }
    val prefs = model.prefs
    SettingsScreen(
        state = SettingsState(
            settings = model.settings,
            theme = prefs.theme,
            appearance = prefs.appearance,
            tiles = prefs.tiles,
            haptics = prefs.haptics,
            themes = model.themes,
            backupNote = model.backupNote,
        ),
        actions = SettingsActions(
            onName = { model.update(SettingsPatch(displayName = it)) },
            onLength = { model.update(SettingsPatch(length = it)) },
            onRemember = { model.update(SettingsPatch(rememberLast = it)) },
            onMotion = { model.update(SettingsPatch(motion = it)) },
            onNetwork = { model.update(SettingsPatch(network = it)) },
            onTheme = prefs::chooseTheme,
            onAppearance = prefs::chooseAppearance,
            onTiles = prefs::chooseTiles,
            onHaptics = prefs::chooseHaptics,
            onBackup = { save.launch("surmise-backup-${java.time.LocalDate.now()}.json") },
            onRestore = { open.launch(arrayOf("application/json", "application/octet-stream", "text/plain")) },
            onHowTo = { model.show(Route.HowTo) },
            onAbout = { model.show(Route.About) },
        ),
    )
}

/** Whether the system's animator scale lets anything move ("remove animations" sets it to 0). */
private fun systemAnimates(context: Context): Boolean =
    SystemSettings.Global.getFloat(context.contentResolver, SystemSettings.Global.ANIMATOR_DURATION_SCALE, 1f) != 0f

/** Hands a share text to the system's share sheet. */
fun share(context: Context, text: String) {
    val send = Intent(Intent.ACTION_SEND).apply {
        type = "text/plain"
        putExtra(Intent.EXTRA_TEXT, text)
    }
    context.startActivity(Intent.createChooser(send, null).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
}
