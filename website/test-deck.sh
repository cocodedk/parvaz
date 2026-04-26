#!/usr/bin/env bash
# Smoke-test the slide-deck navigation on EN + FA pages.
# Drives headless Chrome with a virtual-time budget large enough for
# defer'd script.js to run and mutate the DOM, then greps the dumped
# DOM for the markers we expect (slides, JS-generated dots, theme
# elements). Exits non-zero on any miss.
#
# Usage:  bash website/test-deck.sh   (from anywhere; resolves dir)

set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PORT="${PORT:-18080}"
URL_EN="http://127.0.0.1:${PORT}/"
URL_FA="http://127.0.0.1:${PORT}/fa/"

python3 -m http.server "$PORT" --directory "$DIR" >/dev/null 2>&1 &
SERVER_PID=$!
trap "kill $SERVER_PID 2>/dev/null || true" EXIT
sleep 0.4

dump() {
    google-chrome \
        --headless=new --no-sandbox --disable-gpu \
        --hide-scrollbars --virtual-time-budget=3000 \
        --dump-dom "$1" 2>/dev/null
}

PASS=0; FAIL=0
assert() {
    # $1 = page-label, $2 = test name, $3 = grep-pattern, $4 = dom (file)
    local label="$1" name="$2" needle="$3" dom_file="$4"
    if grep -q -- "$needle" "$dom_file"; then
        echo "  [$label] ✓ $name"
        PASS=$((PASS+1))
    else
        echo "  [$label] ✗ $name  (missing: $needle)" >&2
        FAIL=$((FAIL+1))
    fi
}

run_suite() {
    local label="$1" url="$2"
    local f="/tmp/parvaz-deck-${label}.html"
    dump "$url" > "$f"
    [[ -s "$f" ]] || { echo "  [$label] ✗ page returned empty DOM" >&2; FAIL=$((FAIL+1)); return; }

    # Static markup expectations
    assert "$label" "11 slides on the page"     'data-slide="11"'   "$f"
    assert "$label" "deck-nav present"          'class="deck-nav"'  "$f"
    assert "$label" "side arrows present"       'id="deck-next"'    "$f"
    assert "$label" "solari board renders"      'solari-board'      "$f"
    assert "$label" "boarding pass renders"     'boarding-pass'     "$f"
    assert "$label" "airline-welcome renders"   'airline-welcome'   "$f"
    assert "$label" "lang-switch in nav"        'deck-nav__lang'    "$f"
    assert "$label" "script.js linked"          'script.js'         "$f"

    # Script-generated DOM (runs only if JS executed and IO + dot-gen ran)
    assert "$label" "JS generated 11 dots"      'aria-label="Slide 11"' "$f"
    assert "$label" "current count populated"   'id="deck-current"' "$f"
}

echo "=== EN ==="
run_suite "en" "$URL_EN"
echo "=== FA ==="
run_suite "fa" "$URL_FA"

echo "=== EN ↔ FA parity ==="
parity() {
    local name="$1" pattern="$2"
    local en=$(grep -c -- "$pattern" /tmp/parvaz-deck-en.html)
    local fa=$(grep -c -- "$pattern" /tmp/parvaz-deck-fa.html)
    if [[ "$en" -eq "$fa" ]]; then
        echo "  [parity] ✓ $name  ($en in both)"
        PASS=$((PASS+1))
    else
        echo "  [parity] ✗ $name  (EN=$en FA=$fa)" >&2
        FAIL=$((FAIL+1))
    fi
}
parity "slide count"          'data-slide='
parity "solari cells"         'class="solari-board__cell"'
parity "boarding-pass parts"  'class="boarding-pass'
parity "deck-nav children"    'class="deck-nav__'
parity "section heads"        'class="sec-head"'

echo
echo "passed: $PASS · failed: $FAIL"
[[ $FAIL -eq 0 ]] || exit 1
