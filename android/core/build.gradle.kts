plugins {
    alias(libs.plugins.android.library)
}

android {
    namespace = "com.nxck.surmise.core"
    compileSdk = 37
    defaultConfig { minSdk = 26 }
}

// The Go engine arrives here as libs/core.aar, built by gomobile (P3). The
// directory is gitignored: the .aar is a build output, never committed.
