# TLS flow, part 5: the separate upstream TLS to Apps Script

Moved out of [tls-flow.md](tls-flow.md) to keep that file within the 200-line limit; the text is unchanged.
It is section 5 of the flow described there.

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
