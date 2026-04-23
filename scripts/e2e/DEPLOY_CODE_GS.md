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

Two paths. **Option A (clasp, recommended)** keeps Code.gs in git and
makes every redeploy a scripted one-liner. **Option B (web editor)**
is the manual fallback.

### Option A — clasp (recommended)

`clasp` is Google's official CLI for Apps Script. Makes the script a
first-class versioned artifact instead of a copy-paste blob.

**One-time host setup:**

1. Install Node.js 18+ (check: `node --version`).
2. Install clasp globally:
   ```bash
   npm i -g @google/clasp
   ```
3. Enable the Apps Script API on the test account at
   <https://script.google.com/home/usersettings> — flip the toggle.
   (This is a common gotcha; without it `clasp create` fails with
   a 403.)
4. Log in:
   ```bash
   clasp login
   ```
   Opens a browser for OAuth against the **test account**, writes
   `~/.clasprc.json` on success.

**First deploy:**

1. Set the auth key in `reference/apps_script/Code.gs` before pushing.
   Generate a strong one:
   ```bash
   openssl rand -base64 24 | tr -d '=+/' | head -c 32
   ```
   Replace `CHANGE_ME_TO_A_STRONG_SECRET` with that value. **Don't
   commit the edited Code.gs.** Either keep the change local and
   revert after deploying, or push a second file via clasp only
   (see Option A variant below).
2. Create the standalone Apps Script project bound to the local dir:
   ```bash
   cd reference/apps_script
   clasp create --type standalone --title parvaz-relay --rootDir .
   ```
   Writes `.clasp.json` with the new `scriptId`. The file is
   `.gitignore`d — it's per-account, do not commit it.
3. Push sources up:
   ```bash
   clasp push
   ```
   `appsscript.json` (manifest, committed) + `Code.gs` go up together.
4. Create the first versioned Web App deployment:
   ```bash
   clasp deploy --description "parvaz relay v1"
   ```
5. Grab the deployment ID:
   ```bash
   clasp deployments
   ```
   Output looks like:
   ```
   - AKfycbxXXXXXXXXXXXXXXXXXXX @1 - parvaz relay v1
   ```
   The `AKfycb…` chunk is your `PARVAZ_LIVE_DEPLOYMENT_ID`.
6. On the first invocation, Google asks to grant the
   `https://www.googleapis.com/auth/script.external_request` OAuth
   scope. Open the Web App URL in a browser (test account signed in),
   click through "Go to (unsafe)" — standard for un-verified personal
   scripts.

**Subsequent redeploys:**

```bash
cd reference/apps_script
# edit Code.gs, bump version description
clasp push
clasp deploy --description "parvaz relay v2"
clasp deployments  # copy the new deployment ID if you want to keep the old stable
```

**Handling the auth key cleanly (no leak to git):** keep the committed
Code.gs with the placeholder `AUTH_KEY`. Before `clasp push`, `sed` the
real secret in from `live.env`; after push, `git checkout -- Code.gs`
to restore. A ~10-line wrapper script makes this one command (not
included — trivial to write if you need it).

### Option B — Web editor (fallback)

See [DEPLOY_CODE_GS_WEB_EDITOR.md](DEPLOY_CODE_GS_WEB_EDITOR.md) for
the click-by-click flow. Summary: paste Code.gs into the editor at
<https://script.google.com>, change `AUTH_KEY`, Deploy → New
deployment → Web app → Anyone. Grant OAuth once by running `doPost`
from the editor before hitting the deployment URL.

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
