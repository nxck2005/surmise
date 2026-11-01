package com.nxck.surmise.settings

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.res.pluralStringResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.nxck.surmise.BackupNote
import com.nxck.surmise.R
import com.nxck.surmise.Segmented
import com.nxck.surmise.core.gen.Settings
import com.nxck.surmise.core.gen.ThemeInfo
import com.nxck.surmise.design.Icon
import com.nxck.surmise.design.Radii
import com.nxck.surmise.design.SquircleShape
import com.nxck.surmise.design.SurmiseIcons
import com.nxck.surmise.design.SurmiseText
import com.nxck.surmise.design.SurmiseTheme
import com.nxck.surmise.design.Switch
import com.nxck.surmise.design.TabBarHeight
import com.nxck.surmise.design.hex
import com.nxck.surmise.design.pressable

/** What the settings tab shows: the engine's settings and the phone's own. */
data class SettingsState(
    val settings: Settings,
    val theme: String,
    val appearance: Appearance,
    val tiles: String,
    val haptics: Boolean,
    val themes: List<ThemeInfo>,
    val backupNote: BackupNote? = null,
)

class SettingsActions(
    val onName: (String) -> Unit,
    val onLength: (Int) -> Unit,
    val onRemember: (Boolean) -> Unit,
    val onMotion: (String) -> Unit,
    val onNetwork: (Boolean) -> Unit,
    val onTheme: (String) -> Unit,
    val onAppearance: (Appearance) -> Unit,
    val onTiles: (String) -> Unit,
    val onHaptics: (Boolean) -> Unit,
    val onBackup: () -> Unit,
    val onRestore: () -> Unit,
    val onHowTo: () -> Unit,
    val onAbout: () -> Unit,
)

@Composable
fun SettingsScreen(state: SettingsState, actions: SettingsActions, modifier: Modifier = Modifier) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    val s = state.settings
    Column(
        modifier
            .verticalScroll(rememberScrollState())
            .statusBarsPadding()
            .padding(bottom = TabBarHeight + 48.dp),
    ) {
        SurmiseText(stringResource(R.string.tab_settings), type.display, tokens.ink, Modifier.padding(start = 20.dp, top = 18.dp))

        Section(stringResource(R.string.set_you)) {
            NameRow(s.displayName, actions.onName)
        }
        Note(stringResource(R.string.set_name_note))

        Section(stringResource(R.string.set_play)) {
            Labelled(stringResource(R.string.set_length)) {
                Segmented(listOf(4, 5, 6), s.length.takeIf { it in 4..6 } ?: 5, actions.onLength, Modifier.width(190.dp)) { "$it" }
            }
            Line()
            SwitchRow(stringResource(R.string.set_remember), s.rememberLast, actions.onRemember)
        }
        Note(stringResource(R.string.set_remember_note))

        Section(stringResource(R.string.set_look)) {
            Themes(state.themes, state.theme, actions.onTheme)
            Line()
            Labelled(stringResource(R.string.set_appearance)) {
                Segmented(Appearance.entries, state.appearance, actions.onAppearance, Modifier.width(210.dp)) { appearanceLabel(it) }
            }
            Line()
            Stacked(stringResource(R.string.set_tiles)) {
                Segmented(listOf("", "colorblind", "high-contrast"), state.tiles, actions.onTiles) { tilesLabel(it) }
            }
            Line()
            Stacked(stringResource(R.string.set_motion)) {
                Segmented(listOf("off", "restrained", "pronounced"), s.motion.ifEmpty { "pronounced" }, actions.onMotion) { motionLabel(it) }
            }
            Line()
            SwitchRow(stringResource(R.string.set_haptics), state.haptics, actions.onHaptics)
        }

        Section(stringResource(R.string.set_online)) {
            SwitchRow(stringResource(R.string.set_network), s.network, actions.onNetwork)
        }
        Note(stringResource(R.string.set_network_note))

        Section(stringResource(R.string.set_data)) {
            LinkRow(SurmiseIcons.Download, stringResource(R.string.set_backup), actions.onBackup)
            Line(inset = true)
            LinkRow(SurmiseIcons.Restart, stringResource(R.string.set_restore), actions.onRestore)
        }
        Note(backupText(state.backupNote) ?: stringResource(R.string.set_data_note))

        Section(null) {
            LinkRow(SurmiseIcons.Help, stringResource(R.string.set_howto), actions.onHowTo)
            Line(inset = true)
            LinkRow(SurmiseIcons.Info, stringResource(R.string.set_about), actions.onAbout)
        }
    }
}

@Composable
private fun Section(title: String?, content: @Composable ColumnScope.() -> Unit) {
    val tokens = SurmiseTheme.tokens
    if (title != null) {
        SurmiseText(title, SurmiseTheme.type.caption, tokens.inkMuted, Modifier.padding(start = 32.dp, top = 24.dp, bottom = 8.dp))
    } else {
        Box(Modifier.height(24.dp))
    }
    Column(
        Modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp)
            .background(tokens.card, SquircleShape(Radii.card))
            .padding(horizontal = 16.dp),
        content = content,
    )
}

@Composable
private fun Note(text: String) {
    SurmiseText(text, SurmiseTheme.type.footnote, SurmiseTheme.tokens.inkMuted, Modifier.padding(start = 32.dp, end = 32.dp, top = 8.dp))
}

@Composable
private fun Line(inset: Boolean = false) {
    Box(Modifier.fillMaxWidth().padding(start = if (inset) 32.dp else 0.dp).height(1.dp).background(SurmiseTheme.tokens.ink.copy(alpha = 0.06f)))
}

@Composable
private fun Labelled(label: String, control: @Composable () -> Unit) {
    Row(Modifier.fillMaxWidth().heightIn(min = 52.dp), verticalAlignment = Alignment.CenterVertically) {
        SurmiseText(label, SurmiseTheme.type.body, SurmiseTheme.tokens.ink, Modifier.weight(1f))
        control()
    }
}

@Composable
private fun Stacked(label: String, control: @Composable () -> Unit) {
    Column(Modifier.fillMaxWidth().padding(vertical = 12.dp)) {
        SurmiseText(label, SurmiseTheme.type.body, SurmiseTheme.tokens.ink, Modifier.padding(bottom = 10.dp))
        control()
    }
}

@Composable
private fun SwitchRow(label: String, on: Boolean, onChange: (Boolean) -> Unit) {
    Labelled(label) { Switch(on, onChange) }
}

@Composable
private fun LinkRow(icon: ImageVector, label: String, onClick: () -> Unit) {
    val tokens = SurmiseTheme.tokens
    Row(Modifier.fillMaxWidth().height(52.dp).pressable(onClick = onClick), verticalAlignment = Alignment.CenterVertically) {
        Icon(icon, tokens.inkDim, Modifier.padding(end = 12.dp), size = 20.dp)
        SurmiseText(label, SurmiseTheme.type.body, tokens.ink, Modifier.weight(1f))
        Icon(SurmiseIcons.Chevron, tokens.inkMuted, size = 12.dp)
    }
}

/** The name, saved when the field loses focus or on done. */
@Composable
private fun NameRow(name: String, onName: (String) -> Unit) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    val focus = LocalFocusManager.current
    var text by remember(name) { mutableStateOf(name) }
    fun save() {
        val t = text.trim()
        if (t != name) onName(t)
    }
    Row(Modifier.fillMaxWidth().height(52.dp), verticalAlignment = Alignment.CenterVertically) {
        SurmiseText(stringResource(R.string.set_name), type.body, tokens.ink, Modifier.width(96.dp))
        Box(Modifier.weight(1f), contentAlignment = Alignment.CenterEnd) {
            if (text.isEmpty()) SurmiseText(stringResource(R.string.set_name_hint), type.body, tokens.inkMuted, align = TextAlign.End)
            BasicTextField(
                value = text,
                onValueChange = { if (it.length <= 24) text = it },
                singleLine = true,
                textStyle = type.body.copy(color = tokens.inkDim, textAlign = TextAlign.End),
                cursorBrush = SolidColor(tokens.cursor),
                keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.None, imeAction = ImeAction.Done),
                keyboardActions = KeyboardActions(onDone = { save(); focus.clearFocus() }),
                modifier = Modifier.fillMaxWidth().onFocusChanged { if (!it.isFocused) save() },
            )
        }
    }
}

/** The bundled pair, which the first card ("surmise") stands for. */
private val Bundled = setOf("night", "day")

/** Each theme as a small card in its own colours, with its tiles. */
@Composable
private fun Themes(themes: List<ThemeInfo>, chosen: String, onTheme: (String) -> Unit) {
    val tokens = SurmiseTheme.tokens
    // A pair shows once, by its dark side; a light theme shows alone only
    // when its partner is missing. Night and day are the first card already.
    val shown = themes.filter { t ->
        t.name !in Bundled && (t.pair.isEmpty() || t.dark || themes.none { it.name == t.pair })
    }
    Column(Modifier.padding(vertical = 12.dp)) {
        SurmiseText(stringResource(R.string.set_theme), SurmiseTheme.type.body, tokens.ink, Modifier.padding(bottom = 10.dp))
        LazyRow(horizontalArrangement = Arrangement.spacedBy(10.dp), contentPadding = PaddingValues(end = 4.dp)) {
            item { Swatch(null, chosen.isEmpty()) { onTheme("") } }
            items(shown, key = { it.name }) { t ->
                Swatch(t, chosen == t.name || chosen == t.pair && t.pair.isNotEmpty()) { onTheme(t.name) }
            }
        }
    }
}

@Composable
private fun Swatch(t: ThemeInfo?, on: Boolean, onClick: () -> Unit) {
    val tokens = SurmiseTheme.tokens
    val shape = SquircleShape(Radii.badge)
    Column(horizontalAlignment = Alignment.CenterHorizontally, modifier = Modifier.width(76.dp).pressable(onClick = onClick)) {
        val p = t?.preview
        val canvas = p?.let { runCatching { hex(it.canvas) }.getOrNull() } ?: tokens.canvas
        Box(
            Modifier
                .size(76.dp, 56.dp)
                .background(canvas, shape)
                .border(if (on) 2.dp else 1.dp, if (on) tokens.action else tokens.ink.copy(alpha = 0.1f), shape),
            contentAlignment = Alignment.Center,
        ) {
            Row(horizontalArrangement = Arrangement.spacedBy(3.dp)) {
                val fills = if (p != null) listOf(p.correct, p.present, p.absent).map { runCatching { hex(it) }.getOrNull() } else emptyList()
                val tiles = if (fills.size == 3 && fills.all { it != null }) fills.map { it!! } else listOf(tokens.tiles.correct.fill, tokens.tiles.present.fill, tokens.tiles.absent.fill)
                for (c in tiles) Box(Modifier.size(14.dp).background(c, SquircleShape(3.dp)))
            }
        }
        SurmiseText(
            t?.name ?: stringResource(R.string.set_theme_default),
            SurmiseTheme.type.footnote,
            if (on) tokens.ink else tokens.inkDim,
            Modifier.padding(top = 6.dp),
            maxLines = 1,
        )
    }
}

@Composable
private fun appearanceLabel(a: Appearance) = when (a) {
    Appearance.System -> stringResource(R.string.set_appearance_system)
    Appearance.Light -> stringResource(R.string.set_appearance_light)
    Appearance.Dark -> stringResource(R.string.set_appearance_dark)
}

@Composable
private fun tilesLabel(t: String) = when (t) {
    "colorblind" -> stringResource(R.string.set_tiles_colorblind)
    "high-contrast" -> stringResource(R.string.set_tiles_contrast)
    else -> stringResource(R.string.set_tiles_standard)
}

@Composable
private fun motionLabel(m: String) = when (m) {
    "off" -> stringResource(R.string.set_motion_off)
    "restrained" -> stringResource(R.string.set_motion_calm)
    else -> stringResource(R.string.set_motion_full)
}

@Composable
private fun backupText(n: BackupNote?): String? = when (n) {
    null -> null
    BackupNote.Saved -> stringResource(R.string.set_backup_saved)
    BackupNote.Failed -> stringResource(R.string.set_backup_failed)
    is BackupNote.Restored -> pluralStringResource(R.plurals.set_restored, n.report.puzzlesAdded, n.report.puzzlesAdded)
}
