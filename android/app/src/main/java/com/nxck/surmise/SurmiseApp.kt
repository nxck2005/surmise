package com.nxck.surmise

import android.content.Context
import android.content.Intent
import android.provider.Settings as SystemSettings
import androidx.activity.compose.BackHandler
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
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import com.nxck.surmise.board.BoardScreen
import com.nxck.surmise.daily.DailyScreen
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

/** How long a pushed screen takes to slide in or out. */
private const val PUSH_MS = 320

/** The app: the theme, the tabs, and the screens pushed over them. */
@Composable
fun SurmiseApp(model: AppModel) {
    val context = LocalContext.current
    val dark = isSystemInDarkTheme()
    val tokens = remember(dark) { BundledTokens.forAppearance(context, dark) }
    val motion = Motion(enabled = model.settings.motion != "off" && systemAnimates(context))

    SurmiseTheme(tokens = tokens, motion = motion) {
        LaunchedEffect(Unit) { model.refreshHome() }
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
                )
                Route.Board -> model.board?.let { b ->
                    BoardScreen(b, onBack = { model.closeBoard() }, onHome = model::goHome, onShare = { share(context, it) })
                }
            }
        }
    }
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
                    ),
                    actions = HomeActions(
                        onDaily = model::openDaily,
                        onDailyScreen = model::showDaily,
                        onPlay = model::playLength,
                        onRecord = { model.tab = Tab.Record },
                    ),
                )
                else -> Soon()
            }
        }
        TabBar(
            items = items,
            selected = model.tab.ordinal,
            onSelect = { model.tab = Tab.entries[it] },
            glass = glass,
            modifier = Modifier
                .align(Alignment.BottomCenter)
                .navigationBarsPadding()
                .padding(start = 20.dp, end = 20.dp, bottom = 12.dp),
        )
    }
}

/** A tab not built yet. */
@Composable
private fun Soon() {
    Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
        SurmiseText(stringResource(R.string.tab_soon), SurmiseTheme.type.body, SurmiseTheme.tokens.inkMuted)
    }
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
