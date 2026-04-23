// Top-level build file — configuration shared across sub-projects / modules.
plugins {
    alias(libs.plugins.android.application) apply false
    alias(libs.plugins.kotlin.compose) apply false
}

tasks.register("buildSmoke") {
    group = "verification"
    description = "Debug APK + unit tests + lint — CI-parity smoke check."
    dependsOn(":app:assembleDebug", ":app:testDebugUnitTest", ":app:lintDebug")
}
