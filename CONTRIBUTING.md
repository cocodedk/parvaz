# Contributing to Parvaz

Parvaz is an Android VPN app (Kotlin + Compose) embedding a Go SOCKS5
domain-fronting core as a sidecar binary. Two languages, one repo, one APK.

## Local Setup

1. Install **Android Studio** (latest stable) and let it pull **JDK 17** + Android SDK (min SDK 24, target SDK 36).
2. Install **Go 1.22+** (`go version` to verify).
3. Clone the repo: `git clone https://github.com/cocodedk/parvaz.git && cd parvaz`.
4. Verify the Go core baseline: `go test -C core ./...`.
5. Open `parvaz/` in Android Studio and let it sync Gradle.

## Install Git Hooks

Run once after cloning:

```sh
./scripts/install-hooks.sh
```

The `pre-commit` hook runs `go vet` + `go test` when `.go` files are staged,
and `./gradlew buildSmoke` when `.kt`/`.kts`/`.xml` files are staged. The
`commit-msg` hook enforces Conventional Commits.

## Local Git Setup (recommended)

```sh
git config pull.rebase true
git config core.autocrlf input
git config push.autoSetupRemote true
git config init.defaultBranch main
```

## Build & Test Commands

```sh
# Go core (run from repo root)
go test -C core ./...                 # unit tests
go test -C core -race -cover ./...    # race + coverage
go vet  -C core ./...                 # static checks

# Cross-compile core for Android
CGO_ENABLED=0 GOOS=android GOARCH=arm64 \
    go build -C core -o ../app/src/main/jniLibs/arm64-v8a/libparvaz.so ./cmd/parvazd

# Android app
./gradlew test                        # JVM unit tests
./gradlew assembleDebug               # debug APK
./gradlew assembleRelease             # release APK (needs signing env)
./gradlew buildSmoke                  # CI smoke check

# Live-network tests (needs deployed Code.gs)
PARVAZ_E2E=1 go test -C core ./...
```

## Release signing (one-time, maintainers only)

Release APKs must be signed. Four GitHub repository secrets are required:

| Secret | What it is |
|---|---|
| `KEYSTORE_BASE64` | Base64-encoded release keystore |
| `KEYSTORE_PASSWORD` | Keystore password |
| `KEY_ALIAS` | Signing key alias inside the keystore |
| `KEY_PASSWORD` | Signing key password |

Run the helper once after cloning — it generates or reuses a local keystore,
verifies it with `keytool`, and uploads all four secrets via `gh`:

```sh
./scripts/setup-signing.sh
```

Then trigger a release from the Actions tab (Release workflow → Run workflow)
or via:

```sh
gh workflow run release.yml -f bump=patch   # or minor / major
```

The workflow builds the Go core for 4 Android ABIs, drops the `.so` into
`app/src/main/jniLibs/`, signs the APK, tags `v<version>`, and attaches
`Parvaz.apk` to the GitHub Release.

## Coding Style

- **200-line maximum per file** — extract helpers, composables, or packages when approaching the limit.
- **TDD**: red → green → refactor. One failing test, minimal implementation.
- **Go stdlib first** — `crypto/tls`, `net/http`, `encoding/json`, `compress/gzip`.
- **No panics in Go library code** — return errors; panic only in `main`.
- **No secrets in logs** — never log `auth_key`, deployment URL, or traffic content.
- **Explicit `context.Context`** on every Go network call.
- **State hoisted to ViewModel** in Compose — screens take state + lambdas only.
- **Immutable Kotlin domain models** — `data class` + `copy()`.
- **ViewModels expose `StateFlow`** — `MutableStateFlow` is never public.
- **No Conscrypt hacks in Kotlin** — all TLS / SNI work lives in the Go core.

See `CLAUDE.md` for the full architecture and layer rules.

## Commit Messages — Conventional Commits

Format: `<type>(<optional scope>): <description>`

Types: `feat`, `fix`, `chore`, `docs`, `style`, `refactor`, `test`, `ci`, `build`, `perf`, `revert`

Examples:
- `feat(core/protocol): add batch envelope encoding`
- `feat(app/ui): add rubber-stamp Connect button`
- `fix(core/fronter): honor context cancellation mid-TLS`
- `test(app/settings): EncryptedSharedPreferences round-trip`

The `commit-msg` hook rejects anything that doesn't match.

## Branch Naming

Kebab-case, prefix matches the Conventional Commit type:

| Prefix | Commit type | Example |
|--------|-------------|---------|
| `feature/` | `feat:` | `feature/stamp-button` |
| `fix/` | `fix:` | `fix/context-cancel` |
| `chore/` | `chore:` | `chore/bump-kotlin` |
| `docs/` | `docs:` | `docs/update-architecture` |
| `refactor/` | `refactor:` | `refactor/extract-codec` |
| `ci/` | `ci:` | `ci/add-matrix-abi` |

Never commit directly to `main` — always open a PR.

## PR Checklist

- [ ] `go test -C core -race ./...` passes
- [ ] `./gradlew buildSmoke` passes
- [ ] New logic has unit tests (failing-test-first commit where practical)
- [ ] No file exceeds 200 lines
- [ ] No secrets or personal config committed
- [ ] PR description explains *why*, not just *what*
