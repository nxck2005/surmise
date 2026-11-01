package com.nxck.surmise.design

import android.os.Build
import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.snap
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.runtime.Stable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.unit.dp
import dev.chrisbanes.haze.HazeInput
import dev.chrisbanes.haze.HazeState
import dev.chrisbanes.haze.blur.HazeBlurStyle
import dev.chrisbanes.haze.blur.HazeColorEffect
import dev.chrisbanes.haze.blur.hazeBlur
import dev.chrisbanes.haze.hazeSource

/**
 * What the tab bar blurs. A screen marks its scrolling content with
 * [glassSource] and hands the same state to [TabBar]. It wraps Haze, so
 * nothing outside this module depends on the blur library.
 */
@Stable
class GlassState internal constructor(internal val haze: HazeState)

@Composable
fun rememberGlassState(): GlassState = remember { GlassState(HazeState()) }

/** Marks the content a [TabBar] floats over, so the bar can blur it. */
fun Modifier.glassSource(state: GlassState): Modifier = hazeSource(state.haze)

/** One tab. */
data class TabItem(val label: String, val icon: ImageVector)

val TabBarHeight = 62.dp

/**
 * The floating tab bar: a capsule of frosted glass over the content. The blur
 * needs RenderEffect, so below Android 12, or with no [glass] state, the bar
 * is a near-opaque tint instead, which reads the same at a glance.
 */
@Composable
fun TabBar(
    items: List<TabItem>,
    selected: Int,
    onSelect: (Int) -> Unit,
    modifier: Modifier = Modifier,
    glass: GlassState? = null,
) {
    val tokens = SurmiseTheme.tokens
    val shape = SquircleShape(31.dp)
    val blurs = glass != null && Build.VERSION.SDK_INT >= Build.VERSION_CODES.S
    val surface = if (blurs) {
        Modifier.hazeBlur(
            input = HazeInput.Sources(glass!!.haze),
            style = HazeBlurStyle {
                blurRadius(28.dp)
                noiseFactor(0f)
                colorEffects(listOf(HazeColorEffect.tint(tokens.tabTint.copy(alpha = if (tokens.dark) 0.72f else 0.78f))))
            },
        )
    } else {
        Modifier.background(tokens.tabTint.copy(alpha = if (tokens.dark) 0.94f else 0.96f))
    }
    Row(
        modifier = modifier
            .fillMaxWidth()
            .height(TabBarHeight)
            .shadow(
                elevation = 24.dp,
                shape = shape,
                ambientColor = tokens.shadow.copy(alpha = if (tokens.dark) 0.45f else 0.12f),
                spotColor = tokens.shadow.copy(alpha = if (tokens.dark) 0.45f else 0.12f),
            )
            .clip(shape)
            .then(surface)
            .padding(5.dp),
        horizontalArrangement = Arrangement.spacedBy(4.dp),
    ) {
        items.forEachIndexed { i, item ->
            Tab(item, i == selected, { onSelect(i) }, Modifier.weight(1f))
        }
    }
}

@Composable
private fun Tab(item: TabItem, on: Boolean, onClick: () -> Unit, modifier: Modifier) {
    val tokens = SurmiseTheme.tokens
    val spec = if (SurmiseTheme.motion.enabled) tween<Color>(200, easing = SettleEasing) else snap()
    val fill by animateColorAsState(
        if (on) tokens.ink.copy(alpha = if (tokens.dark) 0.12f else 0.07f) else tokens.ink.copy(alpha = 0f),
        spec,
        label = "tab",
    )
    val ink by animateColorAsState(if (on) tokens.ink else tokens.inkMuted, spec, label = "tab ink")
    Box(
        modifier = modifier
            .fillMaxHeight()
            .pressable(onClick = onClick, role = Role.Tab)
            .clip(SquircleShape(26.dp))
            .background(fill),
        contentAlignment = Alignment.Center,
    ) {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            Icon(item.icon, ink, size = 20.dp)
            Spacer(Modifier.height(3.dp))
            SurmiseText(item.label, SurmiseTheme.type.tab, ink, maxLines = 1)
        }
    }
}
