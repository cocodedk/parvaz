#!/usr/bin/env bash
# Shared helpers for scripts/e2e/*.sh. Source after the per-script
# `timeout` re-exec block (timeout values differ per script).
#
# Exposes:
#   env: ADB, DEVICE, PKG, ROOT, APK, DEPLOY, AUTH, PARVAZ_E2E_HERE
#   log: step, fail, pass, CUR
#   adb: a, sh
#   ui:  dump, find_node, has_id, current_focus, assert_focus

ADB="${ADB:-/home/cocodedk/Android/Sdk/platform-tools/adb}"
DEVICE="${PARVAZ_DEVICE:-}"
PKG="dk.cocode.parvaz"
PARVAZ_E2E_HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$PARVAZ_E2E_HERE/../.." && pwd)"
APK="${APK:-$ROOT/app/build/outputs/apk/debug/app-debug.apk}"

# shellcheck source=/dev/null
[[ -f "$PARVAZ_E2E_HERE/live.env" ]] && source "$PARVAZ_E2E_HERE/live.env"
DEPLOY="${PARVAZ_LIVE_DEPLOYMENT_ID:?set in scripts/e2e/live.env}"
AUTH="${PARVAZ_LIVE_AUTH_KEY:?set in scripts/e2e/live.env}"

if [[ -z "$DEVICE" ]]; then
    DEVICE="$("$ADB" devices | awk '$2=="device" {print $1; exit}')"
    [[ -n "$DEVICE" ]] || { echo "no adb device authorized" >&2; exit 64; }
fi

a()  { "$ADB" -s "$DEVICE" "$@"; }
sh() { a shell "$@"; }

CUR=0
step() { echo "[step $1] $2"; }
fail() { echo "[FAIL step $CUR] $*" >&2; exit 1; }
pass() { echo "[ ok ] $*"; }

# One shell round-trip — `adb pull` would be a second USB hop wasting
# ~150ms per dump.
dump() {
    a exec-out "uiautomator dump /sdcard/ui.xml >/dev/null && cat /sdcard/ui.xml" > /tmp/ui.xml
}

# $1=attr key, $2=value, $3=eq|contains. Echoes "cx cy" on hit, exit 1 on miss.
find_node() {
    python3 - "$1" "$2" "$3" <<'PY'
import re, sys
key, value, mode = sys.argv[1], sys.argv[2], sys.argv[3]
with open('/tmp/ui.xml') as f:
    xml = f.read()
# Match both self-closing (`<node ... />`) and container opening tags
# (`<node ...>...children...</node>`). A `<node([^/]+?)/>` regex would
# silently skip every Compose container, missing testTag-bearing parents
# like `import_field` and `import_submit_button`.
for m in re.finditer(r'<node([^>]+?)/?>', xml):
    attrs = dict(re.findall(r'([\w-]+)="([^"]*)"', m.group(1)))
    av = attrs.get(key, '')
    matched = (av == value) if mode == 'eq' else (value in av)
    if not matched:
        continue
    bm = re.match(r'\[(\d+),(\d+)\]\[(\d+),(\d+)\]', attrs.get('bounds', ''))
    if bm:
        x1, y1, x2, y2 = map(int, bm.groups())
        print((x1 + x2) // 2, (y1 + y2) // 2)
        sys.exit(0)
sys.exit(1)
PY
}

has_id() {
    dump
    find_node "resource-id" "$1" "eq" >/dev/null
}

current_focus() { sh dumpsys window | grep -m1 mCurrentFocus; }

assert_focus() {
    local needle="$1" attempts="${2:-2}"
    for _ in $(seq 1 "$attempts"); do
        focus=$(current_focus)
        [[ "$focus" == *"$needle"* ]] && { pass "focus → $needle"; return 0; }
    done
    fail "expected focus to contain '$needle', got: $(current_focus)"
}
