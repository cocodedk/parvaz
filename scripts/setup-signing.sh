#!/usr/bin/env bash
# scripts/setup-signing.sh
#
# One-time setup: generate (or reuse) a release keystore and upload all four
# signing secrets to the GitHub repo. Required before Release workflow can
# produce a signed APK.
#
# Prerequisites: keytool (JDK), gh CLI authenticated with admin rights.

set -euo pipefail

REPO="cocodedk/parvaz"
KEYSTORE="release.keystore"
ALIAS="parvaz-release"

die() { echo "ERROR: $*" >&2; exit 1; }

command -v keytool >/dev/null || die "keytool not found — install a JDK."
command -v gh      >/dev/null || die "gh not found — install GitHub CLI."
gh auth status >/dev/null 2>&1 || die "gh not authenticated — run 'gh auth login'."

cd "$(git rev-parse --show-toplevel)"

# ── Passwords (read without echoing; reject passwords containing " or \) ───
read_pw() {
  local prompt="$1" __out var
  while :; do
    printf "%s" "$prompt" >&2
    stty -echo; read -r var; stty echo; printf "\n" >&2
    case "$var" in
      *'"'*) echo "Reject: password contains '\"' (breaks Gradle properties)." >&2 ;;
      *'\\'*) echo "Reject: password contains '\\' (breaks Gradle properties)." >&2 ;;
      '')    echo "Reject: empty password." >&2 ;;
      *) __out=$var; break ;;
    esac
  done
  printf "%s" "$__out"
}

# ── Keystore: reuse or generate ────────────────────────────────────────────
if [ -f "$KEYSTORE" ]; then
  echo "Found existing ./$KEYSTORE — reusing."
  STORE_PW=$(read_pw "Keystore password: ")
  KEY_PW=$(read_pw  "Key password (alias=$ALIAS): ")
  keytool -list -v -keystore "$KEYSTORE" -storepass "$STORE_PW" \
    -alias "$ALIAS" >/dev/null \
    || die "keytool -list failed — check passwords / alias."
else
  echo "Generating new keystore ./$KEYSTORE (alias=$ALIAS)"
  STORE_PW=$(read_pw "Choose keystore password: ")
  KEY_PW=$(read_pw  "Choose key password (alias=$ALIAS): ")
  read -r -p "Distinguished Name (CN=Parvaz,O=Cocode,C=DK): " DN
  DN=${DN:-"CN=Parvaz,O=Cocode,C=DK"}
  keytool -genkey -v \
    -keystore "$KEYSTORE" -alias "$ALIAS" \
    -keyalg RSA -keysize 4096 -validity 10000 \
    -storepass "$STORE_PW" -keypass "$KEY_PW" \
    -dname "$DN"
  echo "Verifying new keystore:"
  keytool -list -v -keystore "$KEYSTORE" -storepass "$STORE_PW" \
    -alias "$ALIAS" | head -20
fi

# ── Upload four secrets ────────────────────────────────────────────────────
echo
echo "Uploading secrets to $REPO:"
base64 -w 0 "$KEYSTORE" | gh secret set KEYSTORE_BASE64   --repo "$REPO"
printf "%s" "$STORE_PW" | gh secret set KEYSTORE_PASSWORD --repo "$REPO"
printf "%s" "$ALIAS"    | gh secret set KEY_ALIAS         --repo "$REPO"
printf "%s" "$KEY_PW"   | gh secret set KEY_PASSWORD      --repo "$REPO"
echo "  ✓ KEYSTORE_BASE64"
echo "  ✓ KEYSTORE_PASSWORD"
echo "  ✓ KEY_ALIAS"
echo "  ✓ KEY_PASSWORD"

# ── Safety reminders ───────────────────────────────────────────────────────
if ! git check-ignore -q "$KEYSTORE" 2>/dev/null; then
  echo
  echo "⚠ WARNING: ./$KEYSTORE is not git-ignored. Add to .gitignore NOW."
fi

echo
echo "Release workflow is now ready. Trigger manually:"
echo "  gh workflow run release.yml --repo $REPO -f bump=patch"
echo "Or via the Actions tab → Release → Run workflow."
