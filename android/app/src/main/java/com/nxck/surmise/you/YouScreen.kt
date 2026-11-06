package com.nxck.surmise.you

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import com.nxck.surmise.R
import com.nxck.surmise.Segmented
import com.nxck.surmise.core.gen.ProfileView
import com.nxck.surmise.design.Icon
import com.nxck.surmise.design.SurmiseIcons
import com.nxck.surmise.design.SurmiseText
import com.nxck.surmise.design.SurmiseTheme
import com.nxck.surmise.home.RoundButton
import com.nxck.surmise.record.playtimeText

/**
 * The you tab: who you are and how long you have played, the settings gear,
 * and one switch between the record and the history.
 */
@Composable
fun YouScreen(
    profile: ProfileView?,
    history: Boolean,
    onHistory: (Boolean) -> Unit,
    onSettings: () -> Unit,
    record: @Composable () -> Unit,
    list: @Composable () -> Unit,
    modifier: Modifier = Modifier,
) {
    val tokens = SurmiseTheme.tokens
    val type = SurmiseTheme.type
    Column(modifier.fillMaxSize().statusBarsPadding()) {
        Row(Modifier.fillMaxWidth().padding(start = 20.dp, end = 20.dp, top = 18.dp), verticalAlignment = Alignment.Bottom) {
            Column(Modifier.weight(1f)) {
                if (profile != null && profile.playtimeMs > 0) {
                    SurmiseText(stringResource(R.string.record_playtime, playtimeText(profile.playtimeMs)), type.caption, tokens.inkMuted)
                }
                val name = profile?.displayName?.takeIf { it.isNotBlank() } ?: stringResource(R.string.tab_you)
                SurmiseText(name, type.display, tokens.ink, Modifier.padding(top = 2.dp), maxLines = 1)
            }
            Box(Modifier.padding(bottom = 4.dp)) {
                RoundButton(stringResource(R.string.tab_settings), onSettings) {
                    Icon(SurmiseIcons.Settings, tokens.ink, size = 20.dp)
                }
            }
        }
        Segmented(listOf(false, true), history, onHistory, Modifier.padding(start = 16.dp, end = 16.dp, top = 14.dp)) {
            stringResource(if (it) R.string.you_history else R.string.you_record)
        }
        Box(Modifier.weight(1f)) { if (history) list() else record() }
    }
}
