# SSO verification map

This directory is the maintained source for verifying the user-facing behavior of
ITPRJ-0007 SSO. Read the index before driving the app, then use the matching
feature file as the recipe.

## Baseline preconditions

- Launch the disposable stack with `.agents/skills/verify-sso/scripts/stack.sh up`
  and wait for `verify stack ready on http://localhost:8093`.
- Run `stack.sh doctor`; require `healthz: 200`, `readyz: 200`, and the recorded
  PID alive.
- The stack seeds `user@example.com` / `E2E-User-Passw0rd!` (member) and
  `admin@example.com` / `E2E-Admin-Passw0rd!` (platform-admin).
- The Playwright `setup` project enrolls the admin's TOTP before any spec runs and
  writes the state to `web/e2e/.auth/`.
- Never drive a stack another run started. `stack.sh up` owns the stack it
  records in `.audit/verify/stack.env`.

## Driving conventions

- Start every recipe from the baseline state unless its preconditions say otherwise.
- Prefer ARIA roles and accessible names over CSS selectors or DOM position. The
  suite already uses these handles; reuse them.
- Treat every command as literal. Substitute the port only when `E2E_PORT` was set.
- Run browser actions through Playwright (`cd web && E2E_BASE_URL=... pnpm exec playwright test <spec>`).
- Use `e2e/login.ts`, `e2e/totp.ts`, and `e2e/config.ts` instead of rebuilding
  sign-in, TOTP, or fixture identities.
- Read outbound email at `http://localhost:8025/api/v2/messages`.
- Read state in Postgres with the database named in `.audit/verify/stack.env`.

## Proof and skip reporting

- Capture the user action and the resulting state, not only the final screen.
- UI proof includes a screenshot or trace plus the assertion that the end state
  appeared.
- Mutation proof includes a second, independent read: a database row, a MailHog
  message, or an API response.
- Record the feature ID and entry point used with every artifact.
- Report an unreachable path with the attempted command and the unmet precondition.
- Do not report a skipped entry point as verified through a different path.

## Feature entry contract

Each feature file starts with an H1 title and one paragraph describing the
user-visible behavior. It then uses exactly four H2 sections in this order.

1. `Sub-features` lists short IDs with one line for each behavior.
2. `How to get to it (user POV)` lists every user entry point.
3. `Driving it with Playwright` starts with `Preconditions:` and pairs each user
   action with an exact command or handle and an observable result.
4. `Gotchas` lists traps that can waste or invalidate a verification run.

## Features

- [Sign in and session](./sign-in-and-session.md) covers password login, redirects, session persistence, sign-out, and the session API.
- [Account and security](./account-security.md) covers profile edits, password change, session revocation, and self-service TOTP.
- [Admin console](./admin-console.md) covers the admin gate, users, clients, roles, groups, sessions, audit, and realm settings.
- [OIDC consent](./oidc-consent.md) covers the PKCE authorization request, consent, callback, and logout.
- [Email flows](./email-flows.md) covers password reset, invitation, and email verification through MailHog.
