package com.nxck.surmise.you

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import com.nxck.surmise.R
import com.nxck.surmise.Segmented

/** The you tab: the record or the history, one switch between them. */
@Composable
fun YouScreen(
    history: Boolean,
    onHistory: (Boolean) -> Unit,
    record: @Composable () -> Unit,
    list: @Composable () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier.fillMaxSize().statusBarsPadding()) {
        Segmented(listOf(false, true), history, onHistory, Modifier.padding(start = 16.dp, end = 16.dp, top = 12.dp)) {
            stringResource(if (it) R.string.you_history else R.string.you_record)
        }
        Box(Modifier.weight(1f)) { if (history) list() else record() }
    }
}
