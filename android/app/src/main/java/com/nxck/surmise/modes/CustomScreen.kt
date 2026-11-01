package com.nxck.surmise.modes

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.nxck.surmise.BackButton
import com.nxck.surmise.Page
import com.nxck.surmise.R
import com.nxck.surmise.Segmented
import com.nxck.surmise.design.CapsuleButton
import com.nxck.surmise.design.Card
import com.nxck.surmise.design.MeshGlow
import com.nxck.surmise.design.SquircleShape
import com.nxck.surmise.design.SurmiseText
import com.nxck.surmise.design.SurmiseTheme
import com.nxck.surmise.design.Switch
import com.nxck.surmise.refusalText

/**
 * A word you pick for someone else: typed hidden, checked by the engine,
 * then handed over. The word lives only in this screen's state until
 * [onReady]; nothing saves it, and it is not kept across a process death.
 */
@Composable
fun CustomScreen(
    startLength: Int,
    note: Throwable?,
    onCheck: (word: String, length: Int, anyWord: Boolean, then: () -> Unit) -> Unit,
    onReady: (word: String, length: Int, anyWord: Boolean) -> Unit,
    onBack: () -> Unit,
) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    val context = LocalContext.current
    var length by rememberSaveable { mutableIntStateOf(startLength) }
    var anyWord by rememberSaveable { mutableStateOf(false) }
    var word by remember { mutableStateOf("") }
    var handing by remember { mutableStateOf(false) }

    if (handing) {
        PassThePhone(length, onBack = { handing = false }) {
            val w = word
            word = ""
            onReady(w, length, anyWord)
        }
        return
    }

    Page(stringResource(R.string.custom), onBack) {
        SurmiseText(stringResource(R.string.custom_about), type.body, tokens.inkDim, Modifier.padding(start = 20.dp, end = 20.dp, top = 4.dp))
        Card(Modifier.fillMaxWidth().padding(start = 16.dp, end = 16.dp, top = 20.dp)) {
            Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
                Segmented(listOf(4, 5, 6), length, { length = it; word = word.take(it) }) { stringResource(R.string.letters, it) }
                Box(
                    Modifier.fillMaxWidth().height(52.dp).background(tokens.canvas.copy(alpha = 0.5f), SquircleShape(12.dp)).padding(horizontal = 14.dp),
                    contentAlignment = Alignment.CenterStart,
                ) {
                    if (word.isEmpty()) SurmiseText(stringResource(R.string.custom_hint, length), type.body, tokens.inkMuted)
                    BasicTextField(
                        value = word,
                        onValueChange = { t -> word = t.filter { it.isLetter() }.lowercase().take(length) },
                        singleLine = true,
                        textStyle = type.title.copy(color = tokens.ink, letterSpacing = type.title.fontSize * 0.3f),
                        cursorBrush = SolidColor(tokens.cursor),
                        visualTransformation = PasswordVisualTransformation(),
                        keyboardOptions = KeyboardOptions(
                            capitalization = KeyboardCapitalization.None,
                            autoCorrectEnabled = false,
                            keyboardType = KeyboardType.Password,
                        ),
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Column(Modifier.weight(1f)) {
                        SurmiseText(stringResource(R.string.custom_any), type.body, tokens.ink)
                        SurmiseText(stringResource(R.string.custom_any_note), type.footnote, tokens.inkMuted)
                    }
                    Switch(anyWord, { anyWord = it })
                }
            }
        }
        note?.let {
            SurmiseText(refusalText(context, it, length), type.body, tokens.danger, Modifier.padding(start = 20.dp, end = 20.dp, top = 12.dp))
        }
        CapsuleButton(
            stringResource(R.string.custom_hand_over),
            { onCheck(word, length, anyWord) { handing = true } },
            Modifier.fillMaxWidth().padding(start = 16.dp, end = 16.dp, top = 20.dp),
            enabled = word.length == length,
        )
    }
}

/** The hand-over: nothing of the word on screen, just who goes next. */
@Composable
private fun PassThePhone(length: Int, onBack: () -> Unit, onReady: () -> Unit) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    Column(Modifier.fillMaxSize().background(tokens.canvas).safeDrawingPadding()) {
        Row(Modifier.fillMaxWidth().height(44.dp).padding(horizontal = 16.dp).padding(top = 8.dp), verticalAlignment = Alignment.CenterVertically) {
            BackButton(onBack)
        }
        Box(Modifier.weight(1f).fillMaxWidth().padding(16.dp), contentAlignment = Alignment.Center) {
            MeshGlow(Modifier.fillMaxWidth().clip(SquircleShape(com.nxck.surmise.design.Radii.hero))) {
                Column(Modifier.padding(28.dp).fillMaxWidth(), horizontalAlignment = Alignment.CenterHorizontally) {
                    SurmiseText(stringResource(R.string.custom_pass), type.display, tokens.ink, align = TextAlign.Center)
                    SurmiseText(
                        stringResource(R.string.custom_pass_note, length),
                        type.body,
                        tokens.inkDim,
                        Modifier.padding(top = 10.dp),
                        align = TextAlign.Center,
                    )
                    CapsuleButton(stringResource(R.string.custom_ready), onReady, Modifier.padding(top = 24.dp))
                }
            }
        }
    }
}
