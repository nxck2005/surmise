plugins {
    alias(libs.plugins.android.library)
    alias(libs.plugins.kotlin.serialization)
}

android {
    namespace = "com.nxck.surmise.core"
    compileSdk = 37
    defaultConfig { minSdk = 26 }
}

// The Go engine, built by scripts/build-android-core.sh into libs/ (gitignored:
// a build output, never committed). A library module cannot bundle a local
// .aar, so it compiles against it here and :app packages it.
val engineAar = file("libs/core.aar")

dependencies {
    compileOnly(files(engineAar))
    implementation(libs.kotlinx.serialization.json)
    implementation(libs.kotlinx.coroutines.android)

    testImplementation(libs.junit)
    testImplementation(libs.kotlinx.coroutines.test)
}

// Gradle never runs Go: a missing engine is a step the developer skipped, and
// the message says which one.
tasks.named("preBuild") {
    val aar = engineAar
    doFirst {
        if (!aar.exists()) {
            throw GradleException(
                "missing ${aar.path}: run scripts/build-android-core.sh from the repository root first",
            )
        }
    }
}
