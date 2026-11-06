package com.nxck.surmise.friends

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import com.nxck.surmise.R
import com.nxck.surmise.design.Card
import com.nxck.surmise.design.Icon
import androidx.compose.ui.graphics.vector.ImageVector
import com.nxck.surmise.design.SurmiseIcons
import com.nxck.surmise.design.SurmiseText
import com.nxck.surmise.design.SurmiseTheme
import com.nxck.surmise.design.TabBarHeight
import com.nxck.surmise.design.pressable

/** The ways to play with someone else. */
enum class Way { Rooms, Challenge, Custom }

/** The friends tab: rooms, challenges and pass-the-phone. */
@Composable
fun FriendsScreen(rooms: Boolean, onWay: (Way) -> Unit, modifier: Modifier = Modifier) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    Column(
        modifier
            .verticalScroll(rememberScrollState())
            .statusBarsPadding()
            .padding(bottom = TabBarHeight + 48.dp),
    ) {
        SurmiseText(stringResource(R.string.tab_friends), type.display, tokens.ink, Modifier.padding(start = 20.dp, top = 18.dp))
        SurmiseText(stringResource(R.string.friends_line), type.body, tokens.inkDim, Modifier.padding(start = 20.dp, end = 20.dp, top = 4.dp))
        Column(Modifier.padding(start = 16.dp, end = 16.dp, top = 18.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            if (rooms) WayRow(SurmiseIcons.Rooms, R.string.rooms, R.string.way_rooms) { onWay(Way.Rooms) }
            WayRow(SurmiseIcons.Share, R.string.challenge, R.string.way_challenge) { onWay(Way.Challenge) }
            WayRow(SurmiseIcons.Custom, R.string.custom, R.string.way_custom) { onWay(Way.Custom) }
        }
    }
}

@Composable
private fun WayRow(icon: ImageVector, title: Int, line: Int, onClick: () -> Unit) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    Card(Modifier.fillMaxWidth().height(64.dp).pressable(onClick = onClick), padding = PaddingValues(horizontal = 16.dp)) {
        Row(Modifier.fillMaxWidth().height(64.dp), verticalAlignment = Alignment.CenterVertically) {
            Icon(icon, tokens.accentText, size = 22.dp)
            Column(Modifier.weight(1f).padding(start = 14.dp)) {
                SurmiseText(stringResource(title), type.body.copy(fontWeight = type.section.fontWeight), tokens.ink, maxLines = 1)
                SurmiseText(stringResource(line), type.footnote, tokens.inkDim, maxLines = 1)
            }
        }
    }
}
