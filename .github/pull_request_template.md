## Summary
<!-- What does this PR do and why? Link the milestone in PLAN.md if applicable. -->

## Changes
-

## Test plan

**Go core (`core/`):**
- [ ] `go vet -C core ./...` passes
- [ ] `go test -C core -race ./...` passes
- [ ] New logic has unit tests

**Android app (`app/`):**
- [ ] `./gradlew buildSmoke` passes (assembleDebug + testDebugUnitTest + lintDebug)
- [ ] New UI has `@Preview` composable and looks right in Android Studio
- [ ] Manual verification on device/emulator (if UI-facing change)

## Files
- [ ] No file exceeds 200 lines (`reference/`, `website/` exempt)
- [ ] No secrets committed (`access_key`, deployment URL, `*.keystore`, `local.properties`)
- [ ] `context.Context` threaded through any new Go network call
- [ ] State hoisted to ViewModel — no `SharedPreferences` access in composables

## Related issues
<!-- Closes #NNN -->
