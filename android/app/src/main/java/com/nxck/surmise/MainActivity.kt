package com.nxck.surmise

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.lifecycle.lifecycleScope
import com.nxck.surmise.design.SurmiseTheme
import kotlinx.coroutines.launch

class MainActivity : ComponentActivity() {
    private var model by mutableStateOf<AppModel?>(null)

    override fun onCreate(savedInstanceState: Bundle?) {
        enableEdgeToEdge()
        super.onCreate(savedInstanceState)
        lifecycleScope.launch {
            model = AppModel(Engine.get(this@MainActivity), lifecycleScope)
        }
        setContent {
            val m = model
            if (m != null) {
                SurmiseApp(m)
            } else {
                // The engine opens in well under a frame's worth of a second
                // on most phones; until then, the bare canvas.
                SurmiseTheme { Box(Modifier.fillMaxSize().background(SurmiseTheme.tokens.canvas)) }
            }
        }
    }

    override fun onStop() {
        super.onStop()
        model?.onStop()
    }
}
