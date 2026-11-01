package com.nxck.surmise.modes

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.nxck.surmise.Page
import com.nxck.surmise.R
import com.nxck.surmise.Segmented
import com.nxck.surmise.design.CapsuleButton
import com.nxck.surmise.design.CapsuleStyle
import com.nxck.surmise.design.Card
import com.nxck.surmise.design.SquircleShape
import com.nxck.surmise.design.SurmiseText
import com.nxck.surmise.design.SurmiseTheme
import com.nxck.surmise.refusalText

/** What the challenge screen holds: a code just made, and the last refusal. */
data class ChallengeState(val made: String = "", val madeLength: Int = 0, val note: Throwable? = null)

/**
 * Challenges: make a code to send someone, who plays the same word, or
 * type in a code someone sent.
 */
@Composable
fun ChallengeScreen(
    state: ChallengeState,
    startLength: Int,
    onMake: (Int) -> Unit,
    onShare: (String) -> Unit,
    onOpen: (String) -> Unit,
    onBack: () -> Unit,
) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    val context = LocalContext.current
    var length by rememberSaveable { mutableIntStateOf(startLength) }
    var code by rememberSaveable { mutableStateOf("") }
    Page(stringResource(R.string.challenge), onBack) {
        SurmiseText(stringResource(R.string.challenge_about), type.body, tokens.inkDim, Modifier.padding(start = 20.dp, end = 20.dp, top = 4.dp))

        SurmiseText(stringResource(R.string.challenge_make), type.section, tokens.ink, Modifier.padding(start = 20.dp, top = 24.dp, bottom = 10.dp))
        Card(Modifier.fillMaxWidth().padding(horizontal = 16.dp)) {
            Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
                Segmented(listOf(4, 5, 6), length, { length = it }) { stringResource(R.string.letters, it) }
                if (state.made.isNotEmpty() && state.madeLength == length) {
                    SurmiseText(state.made, type.title, tokens.ink, Modifier.fillMaxWidth(), align = TextAlign.Center)
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        CapsuleButton(stringResource(R.string.challenge_play_it), { onOpen(state.made) }, Modifier.weight(1f), style = CapsuleStyle.Secondary)
                        CapsuleButton(stringResource(R.string.share), { onShare(context.getString(R.string.challenge_share, state.made)) }, Modifier.weight(1f))
                    }
                } else {
                    CapsuleButton(stringResource(R.string.challenge_make_code), { onMake(length) }, Modifier.fillMaxWidth())
                }
            }
        }

        SurmiseText(stringResource(R.string.challenge_enter), type.section, tokens.ink, Modifier.padding(start = 20.dp, top = 24.dp, bottom = 10.dp))
        Card(Modifier.fillMaxWidth().padding(horizontal = 16.dp)) {
            Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
                Box(
                    Modifier.fillMaxWidth().height(48.dp).background(tokens.canvas.copy(alpha = 0.5f), SquircleShape(12.dp)).padding(horizontal = 14.dp),
                    contentAlignment = Alignment.CenterStart,
                ) {
                    if (code.isEmpty()) SurmiseText(stringResource(R.string.challenge_hint), type.code, tokens.inkMuted)
                    BasicTextField(
                        value = code,
                        onValueChange = { if (it.length <= 24) code = it.uppercase() },
                        singleLine = true,
                        textStyle = type.code.copy(color = tokens.ink),
                        cursorBrush = SolidColor(tokens.cursor),
                        keyboardOptions = KeyboardOptions(
                            capitalization = KeyboardCapitalization.Characters,
                            autoCorrectEnabled = false,
                            keyboardType = KeyboardType.Ascii,
                            imeAction = ImeAction.Go,
                        ),
                        keyboardActions = KeyboardActions(onGo = { if (code.isNotBlank()) onOpen(code) }),
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
                CapsuleButton(stringResource(R.string.challenge_open), { onOpen(code) }, Modifier.fillMaxWidth(), enabled = code.isNotBlank())
            }
        }
        state.note?.let {
            SurmiseText(refusalText(context, it), type.body, tokens.danger, Modifier.padding(start = 20.dp, end = 20.dp, top = 12.dp))
        }
    }
}
