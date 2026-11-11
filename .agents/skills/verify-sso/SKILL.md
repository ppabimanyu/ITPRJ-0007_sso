---
name: verify-sso
description: Drive the ITPRJ-0007 SSO app the way a user does — sign-in, account security, admin console, OIDC consent, and email flows — against a throwaway local stack at http://localhost:8093, and capture proof. Use when a change touches the Go server, the Vue SPA, OIDC endpoints, sessions/MFA, admin screens, or outbound email and you must show real behavior rather than test output.
---

# Verify the SSO app

ITPRJ-0007 is a self-hosted SSO service: a Go API plus a Vue 3 SPA served by the
same binary. The primary user surface is the SPA on one origin; the JSON API
lives under `/api/v1` and the OIDC provider under `/realms/{realm}` (the seeded
realm is `default`). Postgres, Redis, and MailHog back it.

This skill launches a disposable stack, seeds known fixtures, drives a feature
through the real UI or API, and records evidence. Read
[`features/README.md`](features/README.md) before choosing a journey.

## Launch

```bash
bash .agents/skills/verify-sso/scripts/stack.sh up
```

It prints `verify stack ready on http://localhost:8093`. Launch is complete only
when that line appears. First run builds the SPA and Go binary and can take a few
minutes; later runs reuse `web/node_modules` and take about 20 seconds.

What `up` does, all through the repo's own `scripts/e2e-stack.sh`:

- starts `postgres`, `redis`, and `mailhog` with `docker compose` (host ports
  `5433`, `6380`, and MailHog's `1025`/`8025`);
- creates a throwaway database `sso_verify_<port>_<epoch>`, applies migrations,
  and seeds fixtures;
- builds `web/` into `internal/webui/dist/`, builds `bin/e2e-server`, and serves
  on the port.

Fixtures:

| Identity | Value | Roles |
| --- | --- | --- |
| User | `user@example.com` / `E2E-User-Passw0rd!` | `member` |
| Admin | `admin@example.com` / `E2E-Admin-Passw0rd!` | `platform-admin` (TOTP enrolled by the Playwright setup project) |
| Public client | `sample-spa`, redirect `http://localhost:5173/callback` | third-party in this stack |
| Confidential client | `sample-backend` | secret printed once in the stack log |

Do not use `make dev` (`:8080`, shared dev database) or `make e2e` (`:8091`, runs
the suite and tears itself down) for a proof that must stay alive.

The SPA is embedded in the compiled binary. After any change under `web/`,
`internal/`, or `cmd/`, relaunch:

```bash
bash .agents/skills/verify-sso/scripts/stack.sh down
bash .agents/skills/verify-sso/scripts/stack.sh up
```

Ports: override with `E2E_PORT=<port>` before `up`. Never drive a stack another
run started; `stack.sh up` refuses a busy port.

## Doctor

```bash
bash .agents/skills/verify-sso/scripts/stack.sh doctor
```

Read-only. It reports the recorded port, PID, database, health, ownership of the
listening port, the three containers, and whether `web/e2e/.auth/` holds setup
state. A healthy run shows `/healthz` and `/readyz` at `200`, the PID alive, the
port owned by `bin/e2e-server`, and all three containers `Up`.

When anything is off, run `stack.sh down` then `stack.sh up`, and read the log
path the helper prints (`stack.sh doctor` names it). Do not reuse a stack that
failed doctor.

## Drive

The repo ships a Playwright suite in `web/e2e/`. Use it first; it is the
supported harness and it handles CSRF, cookies, and admin TOTP.

```bash
cd web
E2E_BASE_URL=http://localhost:8093 pnpm exec playwright test e2e/user.spec.ts --trace on
```

- The `setup` project always runs first: it signs in as both fixtures and enrolls
  the admin's TOTP, writing `web/e2e/.auth/`. Specs then reuse that state.
- One worker, no parallelism (`fullyParallel: false`).
- Useful specs: `user.spec.ts`, `consent.spec.ts`, `admin-mfa.spec.ts`,
  `admin-users.spec.ts`, `admin-clients.spec.ts`. The feature map names which
  spec covers which journey.
- Run the whole suite only for a broad regression; it leaves the stack running
  when you pass `E2E_BASE_URL`, unlike `bash scripts/e2e-run.sh`.

For a journey with no spec, write a temporary spec in `web/e2e/` and delete it
afterwards. Template:

```ts
import { expect, test } from '@playwright/test'
import { userIdentity } from './config'
import { signInAs } from './login'

test('verify: user edits a profile field', async ({ page }) => {
  await signInAs(page, userIdentity.email, userIdentity.password)
  await expect(page.getByRole('heading', { name: 'Your account', exact: true })).toBeVisible()
  await page.getByLabel('Full name', { exact: true }).fill('Sample User')
  await page.getByRole('button', { name: 'Save profile', exact: true }).click()
  await expect(page.getByText('Profile saved.')).toBeVisible()
  await page.screenshot({ path: process.env.VERIFY_SHOT ?? 'verify-shot.png', fullPage: true })
})
```

`signInAs`, `verifyTotpChallenge`, and `totpCode` already exist in `e2e/`; import
them instead of rebuilding login or TOTP. For an authenticated journey start
with `test.use({ storageState: userStatePath })` (or `adminStatePath`) from
`e2e/config`.

If a journey proves valuable, promote the temporary spec into a real spec in the
suite and add it to the feature map; do not keep temp specs around.

API probes are valid only when the API is the surface under test. The auth
endpoints use double-submit CSRF and an origin check:

```bash
B=http://localhost:8093
TOK=$(curl -s -c /tmp/sso-jar "$B/api/v1/auth/csrf" | python3 -c 'import sys,json;print(json.load(sys.stdin)["csrf_token"])')
curl -s -b /tmp/sso-jar -H "Origin: $B" -H "X-CSRF-Token: $TOK" \
  -H "Content-Type: application/json" -d '{"email":"user@example.com"}' \
  "$B/api/v1/auth/forgot-password"
```

MailHog catches every outbound message. Read it with
`curl -s http://localhost:8025/api/v2/messages`; bodies are quoted-printable, so
decode them (`python3 -c 'import quopri,sys;print(quopri.decodestring(sys.stdin.buffer.read()).decode())'`)
before reading a link. The inbox is shared by every stack on this machine.

Drive the real user path, not internal setters or test-only endpoints. Do not
script the OIDC authorize step with a curl login session; start that flow in a
browser context, as `consent.spec.ts` does.

## Evidence

Write artifacts to a run directory that survives teardown:

```bash
RUN="$(pwd)/.audit/verify/$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$RUN"
cd web
E2E_BASE_URL=http://localhost:8093 VERIFY_SHOT="$RUN/account.png" \
  pnpm exec playwright test e2e/user.spec.ts --trace on 2>&1 | tee "$RUN/playwright.txt"
cp -R test-results "$RUN/" 2>/dev/null || true
```

`.audit/` is gitignored. A proof directory should hold:

- the exact command and its output (`playwright.txt`), including the pass/fail
  count;
- a trace (`trace.zip`) or screenshot showing the action and the resulting
  state, not only the final screen;
- the side effect, read back independently: a row from Postgres, a MailHog
  message, or a `/healthz`/API response.

Read a side effect with the database named in `.audit/verify/stack.env`:

```bash
DB=$(sed -n 's/^DB=//p' .audit/verify/stack.env)
docker compose exec -T postgres psql -U sso -d "$DB" \
  -c 'select email, email_verified_at, disabled_at from "user" order by created_at desc limit 5;'
```

Standard: exercise what a user does, observe the resulting state, and check the
side effect separately. Mocks are acceptable only where production already
isolates the external system (MailHog stands in for SMTP). When a path is a
dry-run or test mode, prove what it actually skips by observing files, network,
or rows instead of trusting its name. A proof that drives one entry point is
incomplete when the feature map lists others.

## Cleanup

```bash
bash .agents/skills/verify-sso/scripts/stack.sh down
```

It kills only the PID it recorded, drops the throwaway database, and leaves the
shared `postgres`/`redis`/`mailhog` containers running. Then delete any temporary
spec you added. Never kill by process name. Cleanup removes instances and scratch
state, never the evidence: `.audit/verify/<run>/` stays.

## Helpers

| Command | Purpose |
| --- | --- |
| `scripts/stack.sh up` | Start the disposable stack and wait for health. Idempotent; refuses a busy port. |
| `scripts/stack.sh down` | Stop the recorded server, drop its database, clear state. |
| `scripts/stack.sh doctor` | Read-only health report. |
| `scripts/stack.sh status` | Print the recorded port, PID, database, and log path. |

State lives in `.audit/verify/stack.env`; the stack log is
`.audit/verify/stack-<port>.log`.

## Gotchas

- The server's embedded SPA is rebuilt only at launch. A `web/` edit is invisible
  until you relaunch.
- The password policy rejects a new password that contains the account's email
  or name. `E2E-User-Passw0rd!` is the seeded password but was rejected as a
  *new* password during setup; pick something like `Verified-Passw0rd!42` for a
  reset or change.
- A TOTP time step is single-use. Reusing a code fails; `e2e/totp.ts` handles
  waiting for a fresh step.
- `web/e2e/.auth/admin-totp.json` must match the database the suite runs against.
  A fresh stack always gets a fresh database, so the setup project re-enrolls.
- MailHog's inbox is shared across stacks on this machine; filter by recipient
  and read newest first.
- Login is rate-limited per IP and account; a long fuzz loop locks out the
  fixture user for a while.
