# TLS flow — how the browser accepts a forged cert

A browser typing `https://netflic.com` on a Parvaz-tunneled phone
completes a normal TLS handshake and shows a lock — no warning. This
doc walks the protocol-level mechanics and calls out where Parvaz
differs from a textbook MITM. ECDSA P-256 end-to-end. Matches the
code in `core/mitm/` and `core/fronter/`.

---

## 1) Client → Parvaz: TLS handshake

The phone's browser sends a ClientHello (SNI `netflic.com`, ALPN `h2`,
`http/1.1`). Traffic hits Parvaz because the Android `VpnService` routes
it to our local SOCKS5 server on :1080. SOCKS5 hands the CONNECT target
off to the interceptor *before* reading the ClientHello.

**Parvaz-specific:** cert selection keys on the **SOCKS5 CONNECT host**,
not the SNI. `Interceptor.Intercept(ctx, rawConn, "netflic.com", 443)`
has the host already. The browser's SNI is still sent and validated, but
we pre-select the leaf before the first ClientHello byte.

In practice SNI always equals the CONNECT host, because we only
advertise `http/1.1` on the server side (§3) — that blocks HTTP/2
connection coalescing, the only way a browser would reuse one TLS conn
for a different host.

---

## 2) Parvaz: mint (or reuse) a leaf cert

The interceptor calls `CA.Sign("netflic.com")` which builds:

```
Subject:       CN=netflic.com
SAN:           DNS:netflic.com           (or IPAddresses:<ip> for IP literals)
Issuer:        CN=Parvaz Root CA
KeyUsage:      DigitalSignature
ExtKeyUsage:   ServerAuth
NotBefore:     now - 1h                  (clock-skew slack)
NotAfter:      now + 365d
PubKey:        ECDSA P-256 (fresh per leaf)
Signed by:     CA's ECDSA P-256 key
```

Leaves are cached by host on the shared `Interceptor` — each host is
signed once and reused across every subsequent connection for the life
of parvazd.

The CA itself is generated on first launch at `<data-dir>/ca/{ca.crt,
ca.key}`, persists across restarts, ECDSA P-256, valid 10 years, marked
`IsCA` with `MaxPathLenZero` (no sub-CAs). The Android app hands the PEM
to `ACTION_MANAGE_CA_CERTIFICATES`; the user installs it once.

---

## 3) Parvaz → client: ServerHello + forged chain

`tls.Server` on the raw socket responds with:

- `ServerHello` — ALPN = `http/1.1` (we hard-pin this; see below)
- `Certificate` — the forged leaf + the Parvaz CA cert
- `CertificateVerify`, `Finished`

Browser validates:

- **Hostname match** — SAN contains `netflic.com` → ✓
- **Trust chain** — leaf signed by CA, CA is in Android's user-root
  store (the user installed it in step 0) → ✓
- **Expiry + KeyUsage + EKU** → ✓

Result: handshake succeeds, connection marked secure, lock icon shown.

### Why ALPN is pinned to `http/1.1`

Chrome and Firefox ALPN-advertise `h2` first. If we negotiated HTTP/2
we'd fail on the first frame — the interceptor reads with
`http.ReadRequest`, which only speaks HTTP/1.1. Pinning `http/1.1` also
eliminates HTTP/2 connection coalescing, which is what would let SNI
diverge from the CONNECT host.

### What Android itself shows

Android 7+ shows a persistent notification whenever a user-root is
installed: *"Network may be monitored by an unknown third party."* The OS
compensates honestly for the fact that the browser's security indicator
can't detect user-CA MITM. Parvaz doesn't and can't suppress it.

---

## 4) Post-handshake: plaintext HTTP in our hands

The browser now sends the decrypted request:

```
GET / HTTP/1.1
Host: netflic.com
Accept-Encoding: gzip
...
```

The interceptor reads it with `http.ReadRequest`, rebuilds the absolute
URL from `(CONNECT host, CONNECT port, path+query)`, and hands the
`protocol.Request` to `Relayer.Do`. Keep-alive loops back to the next
`ReadRequest` under a 120s idle deadline.

`FollowRedirects` on that request is **false**: browsers expect to see
3xx responses themselves so the URL bar updates and cross-origin
`Location` headers resolve correctly. Auto-following on the relay side
would silently lie to the browser about which URL served the content.

---

## 5) Parvaz → Apps Script: the completely separate upstream TLS

`relay.Relay.Do` opens (or reuses) a TLS connection via `fronter.Dialer`
to *Google's edge*, not to `netflic.com`:

```
TCP  →  216.239.38.120:443        (configurable Google edge IP)
TLS  →  SNI = www.google.com      (what DPI sees)
HTTP →  Host: script.google.com   (what Google routes by)
Body →  POST /macros/s/<id>/exec  (Apps Script envelope)
```

This is an entirely independent TLS session with a real Google cert
chain. Nothing about this connection references `netflic.com`. The
envelope body encodes the method, URL, headers, and body of the
original request. Apps Script's `UrlFetchApp` fetches `netflic.com`
server-side and returns `{s, h, b}` — status, headers, base64 body.

`relay.Do` decompresses any `Content-Encoding` on the response, drops
the stale `Content-Length`, and hands a `*protocol.Response` back to
the interceptor, which writes it to the browser's TLS conn.

---

## 6) Failure modes — handshake still succeeds

Even if `netflic.com`:

- doesn't resolve in DNS
- has no routable address
- is behind an outage

the TLS handshake with the **browser** still succeeds. The interceptor
never contacts `netflic.com` during the handshake — the cert is locally
minted, the key is local, the verification anchor is local.

Failure surfaces later, as an HTTP response from us to the browser.
`Relayer.Do` returning an error becomes a `502 Bad Gateway` with body
`parvaz relay error: <reason>`. The browser renders it as a normal
error page on what it still thinks is a secure connection.

---

## 7) What the browser actually verifies

The browser does **not** verify:

- The real server identity of `netflic.com`
- `netflic.com`'s real certificate chain
- Anything about the upstream fronted TLS session

It only verifies:

- "Does this leaf cert have a SAN for `netflic.com`?"
- "Does it chain to a CA I trust?"
- "Is it within its validity window?"

All three are satisfied, so the connection is accepted. That's the
whole mechanism.

---

## 8) Summary

From the browser's TLS stack: connected directly to `netflic.com`, with
a valid cert chaining to a trusted root.

Reality:

- TLS is terminated **on the phone** by Parvaz's interceptor
- Certs are synthetic, minted on demand by a user-installed local CA
- A separate TLS session fronts a POST to `script.google.com` via a
  Google edge IP; request body carries the target URL and payload
- There is no certificate forwarding, reuse, or handover — only local
  re-issuance keyed on the SOCKS5 CONNECT host

Real MITM, honest UX: the phone is doing it to itself, with the user's
informed consent via the Android user-root install flow.

## Cross-references

- `core/mitm/ca.go`, `leaf.go` — CA + per-host leaf signing
- `core/mitm/interceptor.go` — TLS server + keep-alive loop
- `core/fronter/dialer.go` — SNI/Host split for the upstream leg
- `core/relay/relay.go` — Apps Script envelope POST + decode
- `apps_script/Code.gs` — server-side fetch logic
