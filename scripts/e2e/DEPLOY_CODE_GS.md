# Deploying Code.gs for Live E2E Testing

This is the Phase C path — the "real traffic" variant. Complements the
local stub in `README.md` (which covers the hermetic CI-friendly half).

The short version: you need a Google account that is **not your
primary**, deploy `reference/apps_script/Code.gs` as a Web App, and
keep its credentials somewhere secret. Below is the full checklist.

---

## 1. Create the test Google account

Choices — ranked by isolation:

| Option | Isolation | Pain |
|---|---|---|
| Fresh personal Gmail (e.g. `cocodedk+parvaz-test@gmail.com`) | medium — same quota family if you used Gmail `+` aliases; separate account if you genuinely register a new `@gmail.com` | low |
| A separate Google Workspace account on a domain you own | highest — fully separate org, no risk to personal assets | medium (Workspace billing or free tier) |
| Reuse your personal account | none — any ban/rate-limit on the project hits your main account | lowest friction, **not recommended** |

Recommendation: **fresh Gmail** (`@gmail.com`, not `+` alias) unless you
already have a Workspace test tenancy. Name suggestion:
`cocodedk-parvaz-test-YYYY@gmail.com`.

**Do on account creation:**

- [ ] Enable 2FA (TOTP; avoid SMS).
- [ ] Set a strong unique password — in a password manager.
- [ ] Fill out recovery email + phone (Google will lock sparse accounts).
- [ ] Don't add a payment method — keeps you on free tier.

---

## 2. Deploy Code.gs

1. Sign in to the test account at <https://script.google.com>.
2. **New project**. Delete the default `Code.gs` content.
3. Paste the entire contents of `reference/apps_script/Code.gs` from
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

8. Grant OAuth scopes when prompted (first invocation). The script
   needs `https://www.googleapis.com/auth/script.external_request` for
   `UrlFetchApp.fetch`. Accept via the "Go to (unsafe)" link — this
   is normal for un-verified personal scripts.

---

## 3. Build the parvaz:// URL

Format: `parvaz://<deployment-id>/<auth-key>#<display-name>`

Example (swap in your real values):

```
parvaz://AKfycbxXXXXXXXXXXXXXXXXXXX/yourRandomSecretHere#parvaz-live-test
```

Paste that into the app's Import screen (or deliver via a QR code).

---

## 4. Smoke test (before committing anything to CI)

Run the relay's unit-test smoke through the live deployment:

```bash
cd core
go test -run TestRelay_GET_TunnelsThroughStub -v \
    -env "PARVAZ_LIVE_URL=https://script.google.com/macros/s/AKfycb.../exec" \
    -env "PARVAZ_LIVE_KEY=yourRandomSecretHere" \
    ./relay/...
```

*(That exact test is currently stub-only — a live variant lands as
part of the Phase C work; see PLAN.md M17.)*

---

## 5. Quotas and gotchas

- Free personal accounts: ~20,000 URL fetch calls per day. Each
  tunneled HTTP request is one fetch. Sufficient for e2e smoke runs,
  too low for sustained use.
- Apps Script also throttles total script runtime — each invocation
  has a 6-minute ceiling. Parvaz single-mode requests finish in <1s
  normally, so this is not a worry.
- **Never** commit the deployment ID or auth key to git. Store them
  in `.env` (gitignored) or a password manager. Rotate the auth key
  if you suspect leak — just edit Code.gs and redeploy (new version).
- **Apps Script ToS** — the script you deploy is subject to Google's
  Apps Script Terms. Per-account quota violations can suspend the
  script. This is why the test account is isolated from your main.

---

## 6. What to commit in the repo

Nothing personal. The live-E2E harness should read credentials from
environment variables at run time, not from any checked-in file. A
sample `scripts/e2e/live.env.example`:

```sh
# Copy to live.env (gitignored) and fill in.
PARVAZ_LIVE_DEPLOYMENT_ID=AKfycbxXXXXXXXXXXXXXXXXXXX
PARVAZ_LIVE_AUTH_KEY=yourRandomSecretHere
```

Ensure `scripts/e2e/live.env` is in `.gitignore`.
