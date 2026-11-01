plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.compose.compiler)
    alias(libs.plugins.roborazzi)
}

android {
    namespace = "com.nxck.surmise"
    compileSdk = 37
    // r28 or newer aligns the Go library for 16 KB pages; pinned so every
    // machine and CI build the same .so.
    ndkVersion = "30.0.16248370"

    defaultConfig {
        applicationId = "com.nxck.surmise"
        minSdk = 26
        targetSdk = 36
        // The release workflow sets these from the tag (a later PR).
        versionCode = 1
        versionName = "0.0.0-dev"
        ndk { abiFilters += listOf("arm64-v8a", "armeabi-v7a", "x86_64") }
    }

    buildTypes {
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
            // Unsigned until the signing PR: a release build here only proves
            // that R8 accepts the app.
        }
    }

    buildFeatures { compose = true }

    // Stored uncompressed and page-aligned, so the loader maps the Go library
    // in place on 16 KB devices.
    packaging { jniLibs { useLegacyPackaging = false } }

    testOptions {
        unitTests {
            isIncludeAndroidResources = true
            // Robolectric's SDK 36 sandbox reaches into FileDescriptor
            // internals, which JDK 21 keeps closed unless they are exported.
            all { it.jvmArgs("--add-exports=java.base/jdk.internal.access=ALL-UNNAMED") }
        }
    }
}

roborazzi {
    // Baselines are committed, so a screenshot test compares against the
    // recorded frame rather than recording a new one on every machine.
    outputDir.set(file("src/test/screenshots"))
}

dependencies {
    implementation(project(":design"))
    implementation(project(":core"))
    // The bound Go engine. :core compiles against it but cannot bundle a local
    // .aar, so the app packages it (see core/build.gradle.kts).
    implementation(files("../core/libs/core.aar"))

    implementation(platform(libs.compose.bom))
    implementation(libs.compose.foundation)
    implementation(libs.compose.ui)
    implementation(libs.compose.ui.tooling.preview)
    implementation(libs.activity.compose)
    implementation(libs.kotlinx.coroutines.android)
    debugImplementation(libs.compose.ui.tooling)
    debugImplementation(libs.compose.ui.test.manifest)

    testImplementation(platform(libs.compose.bom))
    testImplementation(libs.junit)
    testImplementation(libs.kotlinx.coroutines.test)
    testImplementation(libs.kotlinx.serialization.json)
    testImplementation(libs.robolectric)
    testImplementation(libs.compose.ui.test.junit4)
    testImplementation(libs.roborazzi)
    testImplementation(libs.roborazzi.compose)
    testImplementation(libs.roborazzi.junit.rule)
}
