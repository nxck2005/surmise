package com.nxck.surmise.settings

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import com.nxck.surmise.BackButton
import com.nxck.surmise.R
import com.nxck.surmise.core.gen.Info
import com.nxck.surmise.design.Card
import com.nxck.surmise.design.SurmiseText
import com.nxck.surmise.design.SurmiseTheme
import com.nxck.surmise.design.Tile
import com.nxck.surmise.design.TileState
import com.nxck.surmise.design.pressable
import com.nxck.surmise.tileStateOf

/** A pushed page with a back button and a big title. */
@Composable
private fun Page(title: String, onBack: () -> Unit, content: @Composable ColumnScope.() -> Unit) {
    val tokens = SurmiseTheme.tokens
    Column(
        Modifier
            .fillMaxSize()
            .background(tokens.canvas)
            .safeDrawingPadding()
            .verticalScroll(rememberScrollState())
            .padding(bottom = 24.dp),
    ) {
        Row(Modifier.fillMaxWidth().height(44.dp).padding(horizontal = 16.dp).padding(top = 8.dp), verticalAlignment = Alignment.CenterVertically) {
            BackButton(onBack)
        }
        SurmiseText(title, SurmiseTheme.type.display, tokens.ink, Modifier.padding(start = 20.dp, top = 20.dp, bottom = 8.dp))
        content()
    }
}

@Composable
private fun Para(text: String) {
    SurmiseText(text, SurmiseTheme.type.body, SurmiseTheme.tokens.inkDim, Modifier.padding(start = 20.dp, end = 20.dp, top = 12.dp))
}

/** One example guess with its marks, and a line on what it shows. */
@Composable
private fun Example(word: String, marks: String, note: String) {
    Card(Modifier.fillMaxWidth().padding(start = 16.dp, end = 16.dp, top = 12.dp)) {
        Column {
            Row(horizontalArrangement = Arrangement.spacedBy(5.dp)) {
                word.forEachIndexed { i, c ->
                    val state = when (marks[i]) {
                        'c' -> tileStateOf("correct")
                        'p' -> tileStateOf("present")
                        'a' -> tileStateOf("absent")
                        else -> TileState.Typed
                    }
                    Tile(c, state, size = 40.dp)
                }
            }
            SurmiseText(note, SurmiseTheme.type.caption, SurmiseTheme.tokens.inkDim, Modifier.padding(top = 10.dp))
        }
    }
}

@Composable
fun HowToScreen(onBack: () -> Unit) {
    Page(stringResource(R.string.set_howto), onBack) {
        Para(stringResource(R.string.howto_goal))
        Para(stringResource(R.string.howto_marks))
        Example("crane", "c....", stringResource(R.string.howto_correct))
        Example("toast", ".p...", stringResource(R.string.howto_present))
        Example("plumb", "...a.", stringResource(R.string.howto_absent))
        Para(stringResource(R.string.howto_repeats))
        Example("geese", "aaccc", stringResource(R.string.howto_repeats_example))
        Para(stringResource(R.string.howto_modes))
    }
}

@Composable
fun AboutScreen(info: Info?, onBack: () -> Unit) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    val context = LocalContext.current
    Page(stringResource(R.string.set_about), onBack) {
        Card(Modifier.fillMaxWidth().padding(start = 16.dp, end = 16.dp, top = 12.dp)) {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Fact(stringResource(R.string.about_version), info?.version ?: "")
                Fact(stringResource(R.string.about_engine), info?.abi ?: "")
                Fact(stringResource(R.string.about_data), com.nxck.surmise.core.CoreHandle.dataDir(context).path)
            }
        }
        Para(stringResource(R.string.about_licence))
        var open by rememberSaveable { mutableStateOf(false) }
        SurmiseText(
            stringResource(if (open) R.string.about_font_hide else R.string.about_font_show),
            type.body,
            tokens.accentText,
            Modifier.padding(start = 20.dp, top = 12.dp).pressable(onClick = { open = !open }),
        )
        if (open) {
            val text = runCatching { context.assets.open("licenses/geist-OFL.txt").bufferedReader().use { it.readText() } }.getOrDefault("")
            SurmiseText(text, type.code, tokens.inkMuted, Modifier.padding(start = 20.dp, end = 20.dp, top = 12.dp))
        }
    }
}

@Composable
private fun Fact(label: String, value: String) {
    Row(Modifier.fillMaxWidth()) {
        SurmiseText(label, SurmiseTheme.type.body, SurmiseTheme.tokens.ink, Modifier.padding(end = 16.dp))
        SurmiseText(value, SurmiseTheme.type.caption, SurmiseTheme.tokens.inkDim, Modifier.weight(1f), align = androidx.compose.ui.text.style.TextAlign.End)
    }
}
