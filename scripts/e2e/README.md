# Parvaz E2E Harness — Emulator + Local Apps Script Stub

Validates the traffic pipeline **end-to-end** on an Android emulator
without hitting real Google Apps Script:

```
host curl ──SOCKS5── adb-forward ──▶ emulator:1080
                                        │
                                        ▼
                            parvazd (SOCKS5 → dispatcher → MITM)
                                        │
                             TLS via fronter (SNI=e2e.parvaz.test)
                                        │
                                        ▼
                      parvaz-apps-stub on 10.0.2.2:8443
                          (Apps Script envelope contract)
                                        │
                                        ▼
                               canned response → curl
```

Covers:

- MITM leaf signing against the generated CA
- Dispatcher routing (catch-all → MITM+relay path)
- Envelope encode/decode round-trip
- Fronter domain fronting with a self-signed cert (no real Google edge)
- Codec (gzip/br/zstd) per canned response flags

Does **not** cover (out of scope for the local harness):

- The real `Code.gs` runtime — it runs on Google's V8. Running it on
  localhost isn't possible as-is; `UrlFetchApp.fetch` et al. are Google
  APIs. If you want real-Code.gs validation, deploy to a test Google
  account and use the live variant (Phase C).
- CA install UX — requires user tap on the system dialog. Either drive
  it manually during the harness run, or use the emulator-root hack
  (`adb root && adb shell su -c "cp ca.der /system/etc/security/cacerts"`)
  which **does not reflect real-device behavior**.
- VPN permission — same story; system-level consent is user-gated.

## Prerequisites

- Go 1.24+ on host
- Android emulator running an arm64-v8a AVD (see CLAUDE.md §Build
  commands). x86_64 emulators translate apps but not ProcessBuilder
  children — the sidecar won't exec.
- `adb` in PATH
- `curl` with SOCKS5 support (standard on most distros)

## Files

- `run.sh` — orchestration. Currently a **scaffold with TODOs** for
  the pieces that aren't wired yet (see below).
- `../../core/cmd/parvaz-apps-stub/` — standalone Apps Script stub
  binary. Ready to use; self-signs a TLS cert on startup.

## Running

Current state — the stub is self-contained and testable on the host.
The full emulator orchestration is **not yet wired**. See the `TODO`
markers in `run.sh`.

To smoke-test just the stub:

```bash
go build -C core -o /tmp/stub ./cmd/parvaz-apps-stub
/tmp/stub -listen 127.0.0.1:18443 -auth-key smoke &
curl -sk https://127.0.0.1:18443/macros/s/X/exec \
     -H 'Content-Type: application/json' \
     -d '{"k":"smoke","m":"GET","u":"https://e2e.parvaz.test/hi","r":true}'
# → {"s":200,"h":{"Content-Type":"text/plain"},"b":"aGVsbG8gZnJvbSBzdHVi"}
# (base64 "hello from stub")
```

## Remaining Work (tracked in PLAN.md)

1. **parvazd flags** — `-front-port <int>` and `-insecure-tls` so the
   sidecar can be pointed at `10.0.2.2:8443` with a self-signed cert.
   Production code stays at `:443` + strict verify; the new flags are
   opt-in.
2. **Emulator orchestration in `run.sh`** — start stub on host
   (port 8443), `adb push` parvazd + CA to emulator, `adb shell` the
   sidecar with the e2e config, `adb forward tcp:1080`, run curl on
   host, assert response matches, teardown.
3. **CA trust on emulator** — for a fully-automated run, document the
   emulator-root hack as an OPTIONAL shortcut but default to a warning
   that the CA-install dialog needs a tap.
4. **GitHub Actions wiring** — `reactivecircus/android-emulator-runner`
   + the harness, gated on `[e2e]` commit trigger to keep CI fast.
