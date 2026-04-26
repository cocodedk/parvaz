# Deploying Code.gs via the Apps Script Web Editor — Fallback

This is the manual path from `DEPLOY_CODE_GS.md`. Use it if you can't
or don't want to install Node + clasp. Recommendation stays with clasp
(primary doc) — this file exists so the flow isn't lost.

## Steps

1. Sign in to the test account at <https://script.google.com>.
2. **New project**. Delete the default `Code.gs` content.
3. Paste the entire contents of `apps_script/Code.gs` from
   this repo.
4. **Change the auth key.** Find the line:

    ```js
    const AUTH_KEY = "CHANGE_ME_TO_A_STRONG_SECRET";
    ```

   Replace the string with a 24–32 char random secret. Generate one:

    ```bash
    openssl rand -base64 24 | tr -d '=+/' | head -c 32
    ```

   Store this value — you will NOT be able to recover it from the
   Apps Script UI without editing the project again.

5. Save the project (Ctrl+S). Name it `parvaz-relay` (or similar —
   you'll see this in the script.google.com URL).
6. **Deploy → New deployment**:
    - Type: **Web app**
    - Description: `parvaz relay vN` (increment for future redeploys)
    - Execute as: **Me (test-account-email)**
    - Who has access: **Anyone**  ← this is intentional; the auth
      key is what gates access, the URL itself is public.
7. Click **Deploy**. Copy the **Web app URL** — it will look like:

    ```
    https://script.google.com/macros/s/AKfycbxXXXXXXXXXXXXXXXXXXX/exec
                                        └─────┬─────────────┘
                                              deployment ID
    ```

   The deployment ID is the `AKfycb...` chunk between `/macros/s/` and
   `/exec`.

8. Grant OAuth scopes — see "Authorization" below.

## Authorization (one-time)

A fresh web-app deployment cannot run until the deployer account has
granted the script's OAuth scopes. Browser visits to `/exec` will
return Google's "access denied" page until this is done.

In the editor:
- Pick `doPost` in the function dropdown.
- Click **Run**.
- Dialog: **Review permissions** → pick the test account → "Google
  hasn't verified this app" → **Advanced** → **Go to parvaz-relay
  (unsafe)** → **Allow**.
- The function runs, probably hits a caught exception inside
  `doPost(e)` because `e` is undefined when triggered manually — that's
  fine. The scope is now granted.

From here onwards, POST requests with a valid auth key get a proper
envelope response.

## Redeploying

Each change to Code.gs needs **Deploy → Manage deployments → Edit
(pencil icon) → Version: New version → Deploy**. The deployment ID is
stable across versions; deployments can be rolled back from the same
menu.

If you break something and want to start clean, you can also **Deploy →
Test deployments** to see the latest pushed state without creating a
versioned deployment. Test deployment URLs differ from production ones
and don't accept anonymous traffic — they only work while signed in.

## Limitations vs clasp

- No diffs — Code.gs lives inside Google, you edit it in the browser.
- Redeploy is click-heavy — at least 5 clicks per version.
- No version control on the script itself — only deployment versions
  Google tracks internally.
- Auth key ends up pasted into the web editor; easy to leak via
  screenshot, screen-share, or tab history.

If any of those bite, switch to clasp — `DEPLOY_CODE_GS.md` walks the
setup.
