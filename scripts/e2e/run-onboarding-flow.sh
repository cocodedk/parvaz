#!/usr/bin/env bash
# Drives Parvaz onboarding as far as a script can on Samsung One UI 6:
# uninstall → install → deep-link → Import → CA install screen →
# Settings hand-off → navigate Samsung's "More security settings →
# Install from device storage → CA certificate" → tap Install anyway.
# Stops there: the next step is a biometric/PIN auth that no script can
# satisfy without root or the user's PIN — the human has to complete
# that and pick parvaz-ca.crt from the file picker. Returns 0 once
# Install anyway is tapped, or [FAIL step N] earlier.
# Pre-reqs: adb device, built debug APK, scripts/e2e/live.env, Compose
# testTagsAsResourceId enabled in MainActivity.

[[ -z "$_PARVAZ_E2E_REEXEC" ]] && \
    exec env _PARVAZ_E2E_REEXEC=1 timeout 40 bash "$0" "$@"

set -e

source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

URL="parvaz://${DEPLOY}/${AUTH}#e2e-test"
LOG="/tmp/parvaz-e2e-$(date +%s).log"

tap_match() {
    local key="$1" value="$2" mode="$3" attempts="${4:-2}" label="$5"
    : "${label:=$key:$mode:$value}"
    for _ in $(seq 1 "$attempts"); do
        dump
        if coords=$(find_node "$key" "$value" "$mode"); then
            sh input tap $coords
            pass "tapped '$label' @ $coords"
            return 0
        fi
    done
    fail "'$label' not found after $attempts attempts"
}

tap_id()   { tap_match "resource-id" "$1" "eq" "${2:-2}" "id=$1"; }
tap_text() { tap_match "text"        "$1" "eq" "${2:-2}" "text='$1'"; }

# Samsung's Security & privacy + More security settings rows aren't on
# screen on first paint — swipe up until the target appears. Each swipe
# burns ~300ms; bail at 4 attempts so a wrong label fails fast.
tap_text_scroll() {
    local value="$1" attempts="${2:-4}"
    for _ in $(seq 1 "$attempts"); do
        dump
        if coords=$(find_node "text" "$value" "eq"); then
            sh input tap $coords
            pass "tapped (after scroll) text='$value' @ $coords"
            return 0
        fi
        sh input swipe 540 1800 540 700 200
    done
    fail "text='$value' not found after $attempts scroll attempts"
}

CUR=1; step 1 "uninstall + clean install"
a uninstall "$PKG" >/dev/null 2>&1 || true
a install -r "$APK" >/dev/null
pass "installed"

CUR=2; step 2 "wipe logcat + start tail → $LOG"
sh logcat -c
sh logcat -v time | grep --line-buffered -E "CaInstall|CaExporter|CaInstaller|parvazd" > "$LOG" &
TAIL_PID=$!
trap "kill $TAIL_PID 2>/dev/null || true" EXIT

CUR=3; step 3 "deep-link onboarding"
# `am start -d <uri> <pkg>` would treat the trailing pkg as a positional
# URI passed through Intent.parseUri(), clobbering the parvaz:// data URI
# and dropping us on SPLASH. The manifest's parvaz-scheme intent filter
# routes correctly without a package hint.
sh am start -W -a android.intent.action.VIEW -d "'$URL'" | grep -E "Status|Activity" || true
assert_focus "$PKG/$PKG.MainActivity"

CUR=4; step 4 "Import → Continue (id=import_submit_button)"
tap_id "import_submit_button"

CUR=5; step 5 "verify cert exported (CA install screen visible)"
has_id "ca_install_steps" || fail "ca_install_steps never appeared"
pass "ca_install_steps visible"

crt_size=$(sh stat -c %s /storage/emulated/0/Download/parvaz-ca.crt 2>/dev/null | tr -d '\r')
[[ -n "$crt_size" && "$crt_size" -gt 100 ]] \
    || fail "parvaz-ca.crt missing or empty (size='$crt_size')"
pass "parvaz-ca.crt in Downloads (${crt_size} bytes)"

expected_sha=$(a exec-out run-as "$PKG" cat files/parvaz-data/ca/ca.crt \
    | openssl x509 -fingerprint -sha256 -noout 2>/dev/null \
    | cut -d= -f2 | tr -d ':' | tr -d '\n')
[[ -n "$expected_sha" ]] || fail "could not compute expected fingerprint"
pass "expected SHA-256: $expected_sha"

CUR=6; step 6 "tap Open Settings (id=ca_install_primary)"
tap_id "ca_install_primary"
assert_focus "com.android.settings"

CUR=7; step 7 "scroll → 'More security settings'"
# The app fires Settings.ACTION_SECURITY_SETTINGS, which on Samsung One
# UI 6 lands on SecurityAndPrivacySettingsActivity — no global search
# box. Drive the manual path instead.
tap_text_scroll "More security settings"

CUR=8; step 8 "scroll → 'Install from device storage'"
tap_text_scroll "Install from device storage"

CUR=9; step 9 "tap 'CA certificate'"
tap_text "CA certificate"

CUR=10; step 10 "warning dialog → Install anyway"
tap_text "Install anyway"

echo
echo "=== AUTOMATED STEPS DONE ==="
echo "Next on the device (manual): biometric/PIN → file picker → pick"
echo "parvaz-ca.crt → back out of Settings. Parvaz's onResume verify"
echo "should then advance to the VPN permission screen."
echo "logcat: $LOG"
echo "expected fingerprint: $expected_sha"
