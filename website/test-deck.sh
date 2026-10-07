#!/usr/bin/env bash
# Smoke-test the airline-deck navigation on EN + FA + DA pages.
# Drives headless Chrome with a virtual-time budget large enough for
# defer'd script.js to run and mutate the DOM, then greps the dumped
# DOM for the markers we expect (slides, JS-generated dots, deck
# chrome, security alert). Exits non-zero on any miss.
#
# Usage:  bash website/test-deck.sh   (from anywhere; resolves dir)

set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PORT="${PORT:-18080}"
URL_EN="http://127.0.0.1:${PORT}/"
URL_FA="http://127.0.0.1:${PORT}/fa/"
URL_DA="http://127.0.0.1:${PORT}/da/"

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
    local label="$1" name="$2" needle="$3" dom_file="$4"
    if grep -q -- "$needle" "$dom_file"; then
        echo "  [$label] ✓ $name"
        PASS=$((PASS+1))
    else
        echo "  [$label] ✗ $name  (missing: $needle)" >&2
        FAIL=$((FAIL+1))
    fi
}
refute() {
    local label="$1" name="$2" needle="$3" dom_file="$4"
    if grep -q -- "$needle" "$dom_file"; then
        echo "  [$label] ✗ $name  (unexpected: $needle)" >&2
        FAIL=$((FAIL+1))
    else
        echo "  [$label] ✓ $name"
        PASS=$((PASS+1))
    fi
}

run_suite() {
    local label="$1" url="$2"
    local f="/tmp/parvaz-deck-${label}.html"
    dump "$url" > "$f"
    [[ -s "$f" ]] || { echo "  [$label] ✗ page returned empty DOM" >&2; FAIL=$((FAIL+1)); return; }

    # Static markup expectations
    assert "$label" "13 slides on the page"     'data-slide="13"'   "$f"
    assert "$label" "helper deploy listing"     'relay.deploy.001'  "$f"
    assert "$label" "URL extraction diagram"    'AKfycbyLONGRANDOMTOKEN' "$f"
    assert "$label" "airline-deck body class"   'class="airline-deck"' "$f"
    assert "$label" "fixed deck-header"         'class="deck-header"'  "$f"
    assert "$label" "boarding-pass header"      'class="bp-header"'    "$f"
    assert "$label" "airmail band"              'airmail-band'         "$f"
    assert "$label" "MITM trust alert"          'class="deck-alert"'   "$f"
    assert "$label" "alert siren animation"     'deck-alert__siren'    "$f"
    assert "$label" "alert stop sign"           'deck-alert__stop'     "$f"
    assert "$label" "deck-controls present"     'class="deck-controls"' "$f"
    assert "$label" "next arrow present"        'id="deck-next"'       "$f"
    assert "$label" "prev arrow present"        'id="deck-prev"'       "$f"
    assert "$label" "solari board renders"      'solari-board'         "$f"
    assert "$label" "boarding-pass tabs"        'class="bp-tab'        "$f"
    assert "$label" "honest disclosure slide"   'slide--honesty'       "$f"
    assert "$label" "route diagram slide"       'slide--route'         "$f"
    assert "$label" "lang switch in controls"   'deck-controls__lang'  "$f"
    assert "$label" "script.js linked"          'script.js'            "$f"
    assert "$label" "destination is THR"        '>THR<'                "$f"
    refute "$label" "no stale IKA airport code" '>IKA<'                "$f"
    assert "$label" "heads-up advisory present" 'class="heads-up"'     "$f"
    assert "$label" "heads-up tag rendered"     'heads-up__tag'        "$f"
    assert "$label" "websockets caveat visible" 'WebSocket'            "$f"

    # Script-generated DOM (proves JS executed and dot-gen + IO ran)
    # The dots are named in the page language (Persian digits on the Persian page).
    local dot13='aria-label="Slide 13"'
    [[ "$label" == "fa" ]] && dot13='aria-label="اسلاید ۱۳"'
    assert "$label" "JS generated 13 dots"      "$dot13" "$f"
    assert "$label" "current count populated"   'id="deck-current"'    "$f"
    assert "$label" "current mirror populated"  'id="deck-current-mirror"' "$f"
}

echo "=== EN ==="
run_suite "en" "$URL_EN"
echo "=== FA ==="
run_suite "fa" "$URL_FA"
echo "=== DA ==="
run_suite "da" "$URL_DA"

echo "=== nav interactivity (?test=nav) ==="
nav_check() {
    local label="$1" url="$2"
    local f="/tmp/parvaz-deck-nav-${label}.html"
    google-chrome --headless=new --no-sandbox --disable-gpu \
        --hide-scrollbars --window-size=384,800 --virtual-time-budget=5000 \
        --dump-dom "${url}?test=nav" > "$f" 2>/dev/null
    if grep -q 'data-nav-test="pass"' "$f"; then
        echo "  [nav-${label}] ✓ next arrow scrolls one viewport down"
        PASS=$((PASS+1))
    else
        local exp=$(grep -oE 'data-nav-test-expected="[^"]*"' "$f" | head -1)
        local act=$(grep -oE 'data-nav-test-actual="[^"]*"' "$f" | head -1)
        echo "  [nav-${label}] ✗ next arrow did not navigate ($exp $act)" >&2
        FAIL=$((FAIL+1))
    fi
}
nav_check "en" "$URL_EN"
nav_check "fa" "$URL_FA"
nav_check "da" "$URL_DA"

echo "=== stylesheet manifest ==="
css_manifest=$(curl -s "http://127.0.0.1:${PORT}/styles.css")
for partial in 07-deck-base.css 08-deck-slide.css 09-deck-content.css 10-deck-end.css 11-deck-mobile.css; do
    if echo "$css_manifest" | grep -q -- "$partial"; then
        echo "  [css] ✓ $partial imported"
        PASS=$((PASS+1))
    else
        echo "  [css] ✗ $partial not imported via styles.css" >&2
        FAIL=$((FAIL+1))
    fi
done
# Ensure no css partial exceeds the project's 200-line cap.
for f in "$DIR/css"/*.css; do
    lines=$(wc -l < "$f")
    name=$(basename "$f")
    if [[ $lines -le 200 ]]; then
        echo "  [css] ✓ $name ${lines}L (≤200)"
        PASS=$((PASS+1))
    else
        echo "  [css] ✗ $name ${lines}L (over 200-line cap)" >&2
        FAIL=$((FAIL+1))
    fi
done

echo "=== EN ↔ FA parity ==="
parity() {
    # Optional 3rd argument: how many fewer EN may have than FA (default 0).
    local name="$1" pattern="$2" fewer="${3:-0}"
    local en=$(grep -c -- "$pattern" /tmp/parvaz-deck-en.html)
    local fa=$(grep -c -- "$pattern" /tmp/parvaz-deck-fa.html)
    if [[ "$en" -eq $((fa - fewer)) ]]; then
        echo "  [parity] ✓ $name  (EN=$en FA=$fa)"
        PASS=$((PASS+1))
    else
        echo "  [parity] ✗ $name  (EN=$en FA=$fa)" >&2
        FAIL=$((FAIL+1))
    fi
}
parity "slide count"          'data-slide='
parity "solari cells"         'class="solari-board__cell"'
# EN has one tab fewer: its "Download APK" tab became the shared install block
# (cocode-apps), which the Persian page does not carry yet.
parity "boarding-pass tabs"   'class="bp-tab' 1
parity "deck-controls cells"  'class="deck-controls__'
parity "slide heads"          'class="slide__head"'
parity "trust alert blocks"   'class="deck-alert"'

echo "=== EN ↔ DA parity ==="
parity_da() {
    local name="$1" pattern="$2"
    local en=$(grep -c -- "$pattern" /tmp/parvaz-deck-en.html)
    local da=$(grep -c -- "$pattern" /tmp/parvaz-deck-da.html)
    if [[ "$en" -eq "$da" ]]; then
        echo "  [parity] ✓ $name  (EN=$en DA=$da)"
        PASS=$((PASS+1))
    else
        echo "  [parity] ✗ $name  (EN=$en DA=$da)" >&2
        FAIL=$((FAIL+1))
    fi
}
parity_da "slide count"          'data-slide='
parity_da "solari cells"         'class="solari-board__cell"'
parity_da "boarding-pass tabs"   'class="bp-tab'
parity_da "deck-controls cells"  'class="deck-controls__'
parity_da "slide heads"          'class="slide__head"'
parity_da "trust alert blocks"   'class="deck-alert"'
parity_da "shared install block" 'class="install"'

echo
echo "passed: $PASS · failed: $FAIL"
[[ $FAIL -eq 0 ]] || exit 1
