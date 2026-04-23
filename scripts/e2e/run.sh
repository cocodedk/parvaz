#!/usr/bin/env bash
# Parvaz E2E harness — emulator + local Apps Script stub.
# See README.md for what this validates and what remains unimplemented.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

# ────────────────────────────────────────────────────────────────
# Configurable
# ────────────────────────────────────────────────────────────────
STUB_PORT="${STUB_PORT:-8443}"
STUB_SNI="${STUB_SNI:-e2e.parvaz.test}"
STUB_AUTH_KEY="${STUB_AUTH_KEY:-e2e-test-key}"
STUB_ADDR="127.0.0.1:${STUB_PORT}"
STUB_BIN="${ROOT}/build/parvaz-apps-stub"

PARVAZD_PORT="${PARVAZD_PORT:-1080}"
PARVAZD_DATA_DIR="/data/local/tmp/parvaz-e2e"
# On the emulator, the host is reachable via 10.0.2.2 (standard AVD).
EMULATOR_HOST_IP="${EMULATOR_HOST_IP:-10.0.2.2}"

# ────────────────────────────────────────────────────────────────
# Prereqs
# ────────────────────────────────────────────────────────────────
command -v go  >/dev/null || { echo >&2 "ERROR: go not in PATH"; exit 1; }
command -v adb >/dev/null || { echo >&2 "ERROR: adb not in PATH"; exit 1; }
command -v curl >/dev/null || { echo >&2 "ERROR: curl not in PATH"; exit 1; }

adb devices | grep -q "device$" || {
  echo >&2 "ERROR: no Android emulator/device connected (check 'adb devices')"
  exit 1
}

# ────────────────────────────────────────────────────────────────
# Build
# ────────────────────────────────────────────────────────────────
mkdir -p "${ROOT}/build"
echo "▸ building parvaz-apps-stub (host)"
( cd "${ROOT}/core" && go build -o "${STUB_BIN}" ./cmd/parvaz-apps-stub )

# TODO: cross-compile parvazd for android/arm64 and push to emulator.
#   CGO_ENABLED=0 GOOS=android GOARCH=arm64 \
#     go build -C core -o build/parvazd-android ./cmd/parvazd
#   adb push build/parvazd-android /data/local/tmp/parvazd

# ────────────────────────────────────────────────────────────────
# Start stub on host
# ────────────────────────────────────────────────────────────────
echo "▸ starting Apps Script stub on ${STUB_ADDR} (sni=${STUB_SNI})"
"${STUB_BIN}" \
  -listen "${STUB_ADDR}" \
  -auth-key "${STUB_AUTH_KEY}" \
  -sni "${STUB_SNI}" &
STUB_PID=$!
trap 'kill ${STUB_PID} 2>/dev/null || true' EXIT

# Wait for READY
for _ in $(seq 1 20); do
  if curl -sk --max-time 1 "https://${STUB_ADDR}/ping" >/dev/null 2>&1; then
    break
  fi
  sleep 0.1
done

# ────────────────────────────────────────────────────────────────
# TODO: parvazd on emulator
# ────────────────────────────────────────────────────────────────
# Needs two parvazd flags that don't exist yet:
#   -front-port <int>   dial port for the fronter (default 443)
#   -insecure-tls       InsecureSkipVerify on the fronter TLS config
# Then:
#   adb shell "rm -rf ${PARVAZD_DATA_DIR} && mkdir -p ${PARVAZD_DATA_DIR}"
#   adb shell "/data/local/tmp/parvazd -gen-ca -data-dir ${PARVAZD_DATA_DIR}"
#   adb pull ${PARVAZD_DATA_DIR}/ca/ca.crt build/ca-emulator.crt
#   # Either tap through the install dialog (manual) or root-hack the
#   # cert into /system/etc/security/cacerts (emulator-root AVDs only).
#   adb shell "/data/local/tmp/parvazd \
#       -script-urls 'https://${STUB_SNI}/macros/s/STUB1/exec' \
#       -auth-key ${STUB_AUTH_KEY} \
#       -google-ip ${EMULATOR_HOST_IP} \
#       -front-domain ${STUB_SNI} \
#       -front-port ${STUB_PORT} \
#       -insecure-tls \
#       -data-dir ${PARVAZD_DATA_DIR} \
#       -listen-port ${PARVAZD_PORT} &"
#   adb forward tcp:${PARVAZD_PORT} tcp:${PARVAZD_PORT}

# ────────────────────────────────────────────────────────────────
# TODO: exercise the tunnel
# ────────────────────────────────────────────────────────────────
# curl -sk --socks5 127.0.0.1:${PARVAZD_PORT} \
#      https://${STUB_SNI}/hi
# Assert body == "hello from stub"

echo
echo "Scaffold is ready. The stub is running on ${STUB_ADDR}."
echo "TODOs above need landing before this runs fully automated."
echo "Ctrl-C to stop the stub."
wait ${STUB_PID}
