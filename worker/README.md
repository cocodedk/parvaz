# worker/ — Cloudflare Worker deployment

This directory holds Parvaz's server half: a ~50-line Cloudflare Worker
that accepts WebSocket upgrades at `/tunnel` and pipes the frames to
an upstream TCP socket via the `cloudflare:sockets` `connect()` API.

**You — the technical helper — deploy this once.** Non-technical users
receive the resulting `parvaz://<worker>/<access-key>` URL from you via
Telegram and never see the server side.

## One-time setup (5 steps)

1. Install wrangler (Cloudflare's CLI):
   ```sh
   npm install -g wrangler
   ```

2. Log in:
   ```sh
   wrangler login
   ```
   Opens a browser to authenticate against your Cloudflare account.

3. Set your access key. **Pick a strong random string** — it's the shared
   secret between the Worker and every Parvaz APK you share. Either:

   **Option A (simple)** — edit `worker.js` directly:
   ```js
   const ACCESS_KEY = "your-strong-random-string-here";
   ```

   **Option B (safer)** — use Cloudflare's secret store:
   ```sh
   wrangler secret put ACCESS_KEY
   ```
   and replace the `ACCESS_KEY` constant in `worker.js` with `env.ACCESS_KEY`.

4. Deploy:
   ```sh
   npx wrangler deploy
   ```
   Output tells you the worker URL — e.g. `https://parvaz-relay.<you>.workers.dev`.

5. Share the access URL with users:
   ```
   parvaz://parvaz-relay.<you>.workers.dev/<access-key>#نام نمایشی
   ```
   Paste into Telegram. Users scan it or paste it into Parvaz.

## Rotating the access key

Change `ACCESS_KEY` (or run `wrangler secret put ACCESS_KEY`), redeploy,
distribute the new `parvaz://` URL. Old APKs stop working; new URL
unlocks them.

## Limits

Free tier: 100,000 requests/day per worker. Each Parvaz connection is
*one* WebSocket request — so 100K connections/day is the ceiling. For
personal / family use this is vastly more than enough. For larger groups,
upgrade to the $5/mo Paid plan (10 million requests/day).

## Why a Worker, not Apps Script

Apps Script's `UrlFetchApp.fetch()` is URL-based HTTP — it cannot tunnel
opaque TLS bytes. That would force MITM, and on Android MITM breaks
almost every app because apps don't trust user-installed CAs.
Cloudflare's `cloudflare:sockets` API opens raw outbound TCP — bytes pass
through opaque, every app works.

## Source

`worker.js` in this directory — read it. It's 70 lines. Handles
auth, opens the socket, pipes frames, closes cleanly on either side.
No stored state, no database, no logging of traffic.
