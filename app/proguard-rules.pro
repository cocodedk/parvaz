# Add project specific ProGuard rules here.
# You can control the set of applied configuration files using the
# proguardFiles setting in build.gradle.
#
# For more details, see
#   http://developer.android.com/guide/developing/tools/proguard.html

# If your project uses WebView with JS, uncomment the following
# and specify the fully qualified class name to the JavaScript interface
# class:
#-keepclassmembers class fqcn.of.javascript.interface.for.webview {
#   public *;
#}

# Parvaz's failure mode is "the sidecar didn't come up on a phone in
# Iran" — a Log.e with a real line number is the only diagnostic a user
# can screenshot and send back. Worth the small size cost.
-keepattributes SourceFile,LineNumberTable

# If you keep the line number information, uncomment this to
# hide the original source file name.
#-renamesourcefileattribute SourceFile

# Tink (pulled in by androidx.security.crypto's EncryptedSharedPreferences,
# used in ParvazSettings for the access key) references error-prone's and
# JSR-305's annotation classes on classes like KeysetManager and
# PrimitiveSet$Entry. Both annotation libraries are compile-time-only
# (CLASS/SOURCE retention, doc/lint hints) and are never on the runtime
# classpath — the app builds and runs fine without them; R8 just needs
# telling not to warn. List generated from
# app/build/outputs/mapping/release/missing_rules.txt.
-dontwarn com.google.errorprone.annotations.CanIgnoreReturnValue
-dontwarn com.google.errorprone.annotations.CheckReturnValue
-dontwarn com.google.errorprone.annotations.Immutable
-dontwarn com.google.errorprone.annotations.RestrictedApi
-dontwarn javax.annotation.Nullable
-dontwarn javax.annotation.concurrent.GuardedBy