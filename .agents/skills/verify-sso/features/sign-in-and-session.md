# Sign in and session

Password sign-in lets a user reach their account, keeps the session across a
reload, and signs out on request. Wrong credentials produce one generic message
that does not reveal whether the account exists.

## Sub-features

- `login-password` signs in the seeded member with email and password.
- `login-invalid` rejects a wrong password with `Email or password is incorrect.`
- `login-redirect` returns the user to `?redirect=` after sign-in.
- `session-reload` keeps the session and the `Account` link after a page reload.
- `logout` revokes the session and returns to a signed-out view.
- `session-api` returns the current user, roles, and expiry from `GET /api/v1/auth/session`.

## How to get to it (user POV)

- Open `/login` directly.
- Choose `Sign in` from the home page header.
- Open `/account` while signed out; the router sends you to `/login?redirect=/account`.
- Choose `Sign out` on the account page.

## Driving it with Playwright

Preconditions:

- Stack healthy at `http://localhost:8093`; `stack.sh doctor` passes.
- The member fixture exists and `web/e2e/.auth/user.json` is absent or stale.

- **Password login.** Run `cd web && E2E_BASE_URL=http://localhost:8093 pnpm exec playwright test e2e/user.spec.ts --trace on`. Both setup tests and the user spec pass, and the spec ends on `/account` with heading `Your account`.
- **Session survives reload.** The same spec reloads the page and still finds heading `Your account` and link `Account`.
- **Redirect target.** In a temporary spec, `page.goto('/login?redirect=/account')`, fill `Email` and `Password`, choose `Sign in`; the URL becomes `/account`.
- **Invalid password.** Fill `Password` with `not-the-password`, choose `Sign in`; the alert text `Email or password is incorrect.` appears and the URL still starts with `/login`.
- **Sign out.** On `/account`, choose `Sign out` (the first matching button). The page leaves `/account` and the header no longer shows `Account`.
- **Session API.** Reuse the browser context cookie from the spec, or run the API probe in the skill body; `GET /api/v1/auth/session` returns `200` with the fixture email and role `member`.
- **Proof.** Capture the signed-in account page: temporary spec with `await page.screenshot({ path: process.env.VERIFY_SHOT, fullPage: true })` after the `Your account` assertion.

## Gotchas

- Passwords must be at least 12 characters (`PASSWORD_MIN_LENGTH=12`).
- The admin fixture never lands on `/account` from `/login`: MFA enrollment or the
  TOTP challenge comes first. Do not reuse this recipe for the admin.
- Login is rate-limited per IP and per account. Repeated wrong-password probes
  lock the fixture user for a while.
- The password-change flow revokes other sessions; a session you captured earlier
  may be gone after another recipe runs.
