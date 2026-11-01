package com.nxck.surmise

import androidx.compose.foundation.background
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
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.nxck.surmise.design.SurmiseText
import com.nxck.surmise.design.SurmiseTheme

/** A pushed page with a back button and a big title. */
@Composable
fun Page(title: String, onBack: () -> Unit, content: @Composable ColumnScope.() -> Unit) {
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

