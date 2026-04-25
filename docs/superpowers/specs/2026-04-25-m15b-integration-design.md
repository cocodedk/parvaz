# 2026-04-25 — Integrate M15b (tun2socks + DNS) into main, rebase M16 on top

## Why

Phone demo of `m16-error-states` showed Chrome failing with
`ERR_NAME_NOT_RESOLVED` and curl-via-SOCKS5 reporting
`relay: dial tcp 216.239.38.120:443: i/o timeout`. Two architectural
gaps caused this:

1. **Routing loop.** The Go sidecar's outbound TCP is captured by
   the same VpnService that just established the TUN, then forwarded
   back through tun2socks → SOCKS5 → sidecar. Infinite loop.
2. **DNS not tunneled.** tun2socks (M15b-alpha) handled TCP only;
   UDP/53 queries vanished in the TUN.

Both fixes already exist as committed code on feature branches that
were never merged into `main`:

- `feat/android-tun2socks` (`f1c662a`) — M15b-alpha. Routing-loop fix
  via `addDisallowedApplication(packageName)` on `VpnService.Builder`,
  TUN-fd handoff via `Os.fcntlInt(F_SETFD, 0)` + `SidecarConfig.tunFD`.
- `feat/m15b-beta-dns` (`f1d3b2a`) — M15b-beta. SOCKS5 UDP-ASSOCIATE,
  in-sidecar DoH resolver, synthetic `10.0.0.2` DNS target. Two live
  e2e tests on top.

`main` is at `33813c0` (M13b). `m16-error-states` was cut off `main`
*without* M15b underneath, which is why the tunnel can't carry traffic.

## Decision: Path 1 — proper integration

Land M15b on `main` first via a normal PR, then rebase
`m16-error-states` onto the new `main`. Reject the cherry-pick
shortcuts because they preserve the divergence — both branches need
to coexist long-term, not just for tonight's demo.

## Conflict resolution policy

`git merge-tree` simulation shows real text conflicts in three files,
all on the failure-UX axis. M15b-beta predates M16 and removed
machinery that M16 reintroduced. Resolution rules:

| File | M16 says | M15b-beta says | Keep |
|---|---|---|---|
| `ParvazVpnService.kt` | `FailReason` enum + `failReason` on `SessionState` + `hasInternet()` VALIDATED gate + per-reason `fail(reason)` | Plain `fail()`, no enum, no `hasInternet()`, adds `addDisallowedApplication` + FD_CLOEXEC + tun_fd config + `DNS_SERVER = "10.0.0.2"` | **Both.** Keep M16's enum + VALIDATED check; layer M15b-beta's tun_fd / addDisallowed / DNS plumbing on top of `fail(reason)` paths. |
| `ParvazConnectionState.kt` (new in M15b-beta) | n/a (didn't exist) | extracts `ConnectionState` + `SessionState` out of `ParvazVpnService` | **Add `FailReason` here too** so the whole failure-state model lives in one file. M16's import paths shift accordingly. |
| `MainViewModel.kt` | `failReason` field on `MainUiState`; imports `FailReason` from `ParvazVpnService` | drops `failReason` field; imports `ConnectionState` from `ParvazConnectionState` | **M16's `failReason` field, but import `FailReason` from `ParvazConnectionState` (per row above).** |
| `MainScreen.kt` | `failReasonStringRes(reason)` lookup mapping `FailReason` → string res | doesn't exist | **M16's version, with `FailReason` import path updated.** |
| `strings.xml` (FA + EN) | adds 4 `main_failed_*` keys | drops them | **M16's keys.** |

Net effect: the rebased `ParvazVpnService.kt` has tun_fd handoff +
disallowed app + DoH wiring + Android-too-old guard. `ParvazConnectionState.kt` owns
`ConnectionState` + `FailReason` + `SessionState`. The differentiated failure copy
keeps working with one import-path change in M16's downstream files.

## Working-tree state at start

`m16-error-states` has substantial uncommitted work:

- **Brand identity WIP** (drawables + launcher webps + theme/manifest/
  build.gradle for cold splash). Three drawables now staged, the rest
  modified.
- **My recent codex-review fixes**: PLAN.md trim, CaInstallScreen.kt
  refactor (`verify`/`generate` helpers), CoreLauncher.kt coroutine
  conversion + `@Volatile`, INTERNET permission, instrumentation test
  assert fix.

These do NOT belong on the M15b PR — they're independent. Park them on
a recovery branch before the rebase.

## Execution sequence

1. **Save WIP** — commit the dirty working tree to a temporary branch
   `wip/m16-recovery-2026-04-25`. Verify nothing is lost.
2. **Open PR** `feat/m15b-beta-dns` → `main`. The diff is clean against
   today's `main` (M16 isn't in `main` yet). Use `gh pr create`.
3. **Wait for merge** — review by user / codex / CI as normal. Do NOT
   merge autonomously.
4. **Rebase** `m16-error-states` on the new `main`:
   - Conflicts in `ParvazVpnService.kt`, `MainViewModel.kt`,
     `MainScreen.kt`, both `strings.xml`. Resolve per the table above.
   - Run `./gradlew test`, `go test -C core ./...`, instrumentation
     suite, live e2e — all must remain green.
5. **Reapply WIP** from the recovery branch onto rebased
   `m16-error-states`. Cherry-pick or `git restore`-from-branch.
6. **Verify on phone** — install fresh APK, walk CA install + connect,
   open Chrome → google.com / example.com.
7. **Codex review** the rebased `m16-error-states`.

## Risks

- **Wrong conflict resolution** in step 4 silently regresses one of
  the two sets of behaviour (failure-copy granularity OR tun fd
  handoff). Mitigation: codex review post-rebase + on-device test.
- **WIP reapply collides** with M15b's own changes to `ParvazVpnService`
  / `themes.xml` / `AndroidManifest`. Mitigation: review each file
  during the cherry-pick.
- **PR review cycle** stretches the calendar. The demo waits.

## Out of scope for this spec

- Fixing the brand identity WIP (separate PR after M15b lands).
- New M16 work — M16 is functionally done; this only repairs how it
  sits on the tree.
- Changing the M15b protocol design. We're integrating, not redesigning.
