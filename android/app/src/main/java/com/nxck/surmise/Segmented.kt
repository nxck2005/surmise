package com.nxck.surmise

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import com.nxck.surmise.design.CapsuleShape
import com.nxck.surmise.design.SurmiseText
import com.nxck.surmise.design.SurmiseTheme
import com.nxck.surmise.design.pressable

/** A row of choices in one capsule, the chosen one raised. */
@Composable
fun <T> Segmented(values: List<T>, selected: T, onSelect: (T) -> Unit, modifier: Modifier = Modifier, label: @Composable (T) -> String) {
    val tokens = SurmiseTheme.tokens
    Row(modifier.fillMaxWidth().height(36.dp).background(tokens.card, CapsuleShape).padding(3.dp)) {
        for (v in values) {
            val on = v == selected
            Box(
                Modifier
                    .weight(1f)
                    .height(30.dp)
                    .pressable(onClick = { onSelect(v) })
                    .background(if (on) tokens.ink.copy(alpha = 0.12f) else Color.Transparent, CapsuleShape),
                contentAlignment = Alignment.Center,
            ) {
                SurmiseText(label(v), SurmiseTheme.type.caption, if (on) tokens.ink else tokens.inkDim, maxLines = 1)
            }
        }
    }
}
