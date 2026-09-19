plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.compose)
}

// ── Version from gradle.properties ──────────────────────────────────────────
// The version lives in gradle.properties, where the release workflow and
// F-Droid's checkupdates both read it (its reproducible build invokes Gradle
// directly, not through our release workflow, so it needs the value pinned
// in the source rather than passed through the environment). See the comment
// there before bumping it.
val versionNameValue: String = providers.gradleProperty("VERSION_NAME").get()
val versionCodeValue: Int = providers.gradleProperty("VERSION_CODE").get().toInt()

// ── Signing config from env vars (release workflow) ────────────────────────
val signingKeystorePath = System.getenv("KEYSTORE_PATH")?.takeIf { it.isNotBlank() }
val signingKeystorePassword = System.getenv("KEYSTORE_PASSWORD")?.takeIf { it.isNotBlank() }
val signingKeyAlias = System.getenv("KEY_ALIAS")?.takeIf { it.isNotBlank() }
val signingKeyPassword = System.getenv("KEY_PASSWORD")?.takeIf { it.isNotBlank() }
val signingKeystoreFile = signingKeystorePath
    ?.let { rootProject.file(it).absoluteFile }
    ?.takeIf { it.isFile }
val hasSigningConfig = signingKeystoreFile != null &&
    signingKeystorePassword != null &&
    signingKeyAlias != null &&
    signingKeyPassword != null

android {
    namespace = "dk.cocode.parvaz"
    compileSdk {
        version = release(36) {
            minorApiLevel = 1
        }
    }

    // AGP otherwise adds a Google-encrypted dependency list to the APK signing
    // block, and F-Droid rejects any release APK that carries it.
    dependenciesInfo {
        includeInApk = false
        includeInBundle = false
    }

    defaultConfig {
        applicationId = "dk.cocode.parvaz"
        minSdk = 24
        targetSdk = 36
        versionCode = versionCodeValue
        versionName = versionNameValue
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"

        // Parvaz users are on Android phones in Iran — all arm64. Go can
        // cross-compile android/arm64 without cgo; android/amd64 needs
        // cgo + NDK clang, which we don't set up. Forcing arm64-v8a as
        // the only packaged ABI makes Android's installer pick it as the
        // primary ABI on x86_64 emulators too (the emulator's ARM
        // translation layer handles execution; abilist advertises both).
        ndk {
            abiFilters += listOf("arm64-v8a", "x86_64")
        }
    }

    if (hasSigningConfig) {
        signingConfigs {
            create("release") {
                storeFile = signingKeystoreFile!!
                storePassword = signingKeystorePassword
                keyAlias = signingKeyAlias
                keyPassword = signingKeyPassword
            }
        }
    }

    buildTypes {
        release {
            // R8 shrinks the release APK, which matters on the low-bandwidth
            // connections our users have.
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro"
            )
            if (hasSigningConfig) {
                signingConfig = signingConfigs.getByName("release")
            }
        }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_11
        targetCompatibility = JavaVersion.VERSION_11
    }
    buildFeatures {
        compose = true
    }

    // ProcessBuilder needs the Go sidecar binary as a real file on disk, so
    // the APK must ship libparvaz.so extracted instead of memory-mapped.
    // AGP 9 requires setting this here, not via android:extractNativeLibs.
    packaging {
        jniLibs {
            useLegacyPackaging = true
        }
    }
}

// Go core cross-compile for Android lives in .github/workflows/release.yml.
// Locally: see CLAUDE.md §Build commands for the one-liner.

dependencies {
    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.core.splashscreen)
    implementation(libs.androidx.lifecycle.runtime.ktx)
    implementation(libs.androidx.lifecycle.runtime.compose)
    implementation(libs.androidx.lifecycle.viewmodel.compose)
    implementation(libs.androidx.activity.compose)
    implementation(libs.androidx.security.crypto)
    implementation(platform(libs.androidx.compose.bom))
    implementation(libs.androidx.compose.ui)
    implementation(libs.androidx.compose.ui.graphics)
    implementation(libs.androidx.compose.ui.tooling.preview)
    implementation(libs.androidx.compose.material3)
    testImplementation(libs.junit)
    testImplementation(libs.json)
    androidTestImplementation(libs.androidx.junit)
    androidTestImplementation(libs.androidx.test.core)
    androidTestImplementation(libs.androidx.espresso.core)
    androidTestImplementation(platform(libs.androidx.compose.bom))
    androidTestImplementation(libs.androidx.compose.ui.test.junit4)
    debugImplementation(libs.androidx.compose.ui.tooling)
    debugImplementation(libs.androidx.compose.ui.test.manifest)
}
