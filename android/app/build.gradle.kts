plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.compose.compiler)
    alias(libs.plugins.roborazzi)
}

// The release workflow passes the tag's version and its versionCode (see
// release.yml); any other build is 0.0.0-dev.
val surmiseVersion = providers.gradleProperty("surmiseVersion").getOrElse("0.0.0-dev")
val surmiseVersionCode = providers.gradleProperty("surmiseVersionCode").getOrElse("1").toInt()

// The app signing key, from the environment, which only the release workflow
// sets. A release build without it comes out unsigned.
val keystore: String? = System.getenv("SURMISE_KEYSTORE")

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
        versionCode = surmiseVersionCode
        versionName = surmiseVersion
        ndk { abiFilters += listOf("arm64-v8a", "armeabi-v7a", "x86_64") }
    }

    signingConfigs {
        if (keystore != null) {
            create("release") {
                storeFile = file(keystore)
                storePassword = System.getenv("SURMISE_KEYSTORE_PASSWORD")
                keyAlias = System.getenv("SURMISE_KEY_ALIAS")
                keyPassword = System.getenv("SURMISE_KEY_PASSWORD")
            }
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
            if (keystore != null) signingConfig = signingConfigs.getByName("release")
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
    implementation(libs.kotlinx.serialization.json)
    testImplementation(libs.robolectric)
    testImplementation(libs.compose.ui.test.junit4)
    testImplementation(libs.roborazzi)
    testImplementation(libs.roborazzi.compose)
    testImplementation(libs.roborazzi.junit.rule)
}
