#!/usr/bin/env bash
# Verifies CA reuse policy on app update vs full reinstall.
#
#   Phase A (UPDATE — `adb install -r`, /data/data preserved)
#     CA on disk is the same → fingerprint matches the cert sitting in
#     AndroidCAStore → CaInstallScreen.prepare() flips straight to
#     INSTALLED → user lands on VPN permission screen with no taps.
#
#   Phase B (REINSTALL — `adb uninstall && adb install`, data wiped)
#     New CA generated → fingerprint differs from the orphan cert still
#     in the keystore → CaInstallScreen shows the install steps again.
#     Phase B leaves the device needing a fresh manual cert install
#     before Parvaz is usable; pass --skip-reinstall to stop after A.
#
# Pre-req: the app's current CA must already be installed on the device
# (run scripts/e2e/run-onboarding-flow.sh + complete biometric/PIN once).

[[ -z "$_PARVAZ_E2E_REEXEC" ]] && \
    exec env _PARVAZ_E2E_REEXEC=1 timeout 30 bash "$0" "$@"

set -e

source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

URL="parvaz://${DEPLOY}/${AUTH}#e2e-reuse"

SKIP_REINSTALL=0
[[ "${1:-}" == "--skip-reinstall" ]] && SKIP_REINSTALL=1

ca_fingerprint() {
    a exec-out run-as "$PKG" cat files/parvaz-data/ca/ca.crt 2>/dev/null \
        | openssl x509 -fingerprint -sha256 -noout 2>/dev/null \
        | cut -d= -f2 | tr -d ':' | tr -d '\n'
}

deep_link() { sh am start -W -a android.intent.action.VIEW -d "'$URL'" >/dev/null; }

# One dump per attempt, then check both ids against that snapshot —
# avoids the 4-dumps-per-iteration cost of `has_id A || has_id B`.
expect_either_id() {
    local a="$1" b="$2" attempts="${3:-3}"
    for _ in $(seq 1 "$attempts"); do
        dump
        find_node "resource-id" "$a" "eq" >/dev/null && return 0
        find_node "resource-id" "$b" "eq" >/dev/null && return 0
    done
    return 1
}

CUR=1; step 1 "precondition: capture current on-disk CA fingerprint"
PRE_FP="$(ca_fingerprint)"
[[ -n "$PRE_FP" ]] || fail "couldn't read CA from app data — run onboarding first"
pass "pre-update fingerprint: $PRE_FP"

CUR=2; step 2 "PHASE A — update (adb install -r, data preserved)"
a install -r "$APK" >/dev/null
pass "updated"

CUR=3; step 3 "deep-link → expect VPN permission (CA reused, install screen skipped)"
deep_link
expect_either_id "vpn_permission_primary" "vpn_permission_spinner" \
    || fail "still on CA install screen after update — reuse path failed"
pass "VPN permission screen reached → cert was reused"

CUR=4; step 4 "fingerprint unchanged after update"
POST_UPDATE_FP="$(ca_fingerprint)"
[[ "$POST_UPDATE_FP" == "$PRE_FP" ]] \
    || fail "CA changed across update: $PRE_FP → $POST_UPDATE_FP"
pass "fingerprint preserved: $POST_UPDATE_FP"

if [[ $SKIP_REINSTALL -eq 1 ]]; then
    echo
    echo "=== PHASE A PASSED (skipped reinstall) ==="
    exit 0
fi

CUR=5; step 5 "PHASE B — full reinstall (uninstall + install, data wiped)"
a uninstall "$PKG" >/dev/null 2>&1 || true
a install "$APK" >/dev/null
pass "reinstalled"

CUR=6; step 6 "deep-link → expect CA install screen (new fingerprint ≠ keystore)"
deep_link
expect_either_id "ca_install_steps" "ca_install_primary" \
    || fail "expected CA install screen after reinstall, got something else"
pass "CA install screen shown → fresh CA correctly NOT auto-trusted"

CUR=7; step 7 "fingerprint changed across reinstall"
POST_REINSTALL_FP="$(ca_fingerprint)"
[[ -n "$POST_REINSTALL_FP" ]] || fail "couldn't read post-reinstall CA"
[[ "$POST_REINSTALL_FP" != "$PRE_FP" ]] \
    || fail "CA fingerprint unchanged across full reinstall — data wasn't wiped?"
pass "new fingerprint: $POST_REINSTALL_FP"

echo
echo "=== BOTH PHASES PASSED ==="
echo "NOTE: device now has a fresh CA that's NOT in the keystore."
echo "Re-run run-onboarding-flow.sh + complete biometric to restore usable state."
