package com.nxck.surmise

import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import com.nxck.surmise.design.CapsuleShape
import com.nxck.surmise.design.Icon
import com.nxck.surmise.design.SurmiseIcons
import com.nxck.surmise.design.SurmiseText
import com.nxck.surmise.design.SurmiseTheme
import com.nxck.surmise.design.pressable

/** The round back button a pushed screen has in place of the tab bar. */
@Composable
fun BackButton(onClick: () -> Unit, modifier: Modifier = Modifier) {
    val tokens = SurmiseTheme.tokens
    val label = stringResource(R.string.back)
    Box(
        modifier = modifier
            .size(40.dp)
            .pressable(onClick = onClick)
            .border(1.dp, tokens.tileEmpty, CapsuleShape)
            .semantics { contentDescription = label },
        contentAlignment = Alignment.Center,
    ) {
        Icon(SurmiseIcons.Back, tokens.inkDim, size = 14.dp)
    }
}

/**
 * A pushed screen's top bar: back on the left, a title and a quiet second
 * line in the middle, and [trailing] (a clock, say) on the right, in a slot
 * as wide as the back button so the title stays centred.
 */
@Composable
fun TopBar(
    title: String,
    onBack: () -> Unit,
    modifier: Modifier = Modifier,
    subtitle: String? = null,
    trailing: @Composable RowScope.() -> Unit = {},
) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    Row(
        modifier = modifier.fillMaxWidth().height(44.dp).padding(horizontal = 16.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        BackButton(onBack)
        Column(Modifier.weight(1f), horizontalAlignment = Alignment.CenterHorizontally) {
            SurmiseText(title, type.headline, tokens.ink, maxLines = 1)
            if (subtitle != null) SurmiseText(subtitle, type.code, tokens.inkMuted, maxLines = 1)
        }
        Row(Modifier.width(40.dp), horizontalArrangement = Arrangement.End, content = trailing)
    }
}
