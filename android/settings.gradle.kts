pluginManagement {
    repositories {
        google()
        mavenCentral()
        gradlePluginPortal()
    }
}

dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        google()
        mavenCentral()
    }
}

rootProject.name = "surmise-android"

// :app is the screens and navigation, :design every token and shared
// component, :core the bound Go engine. :app never defines a colour, font or
// radius itself; those come from :design.
include(":app", ":design", ":core")
