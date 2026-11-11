# Admin console

The admin console is a platform-admin area gated by a completed MFA challenge.
From it an administrator manages users, OIDC clients, roles, groups, active
sessions, audit events, and realm security settings.

## Sub-features

- `admin-gate` blocks a platform-admin without MFA, forcing enrollment, then the TOTP challenge.
- `admin-dashboard` shows the console at `/admin` with the section navigation.
- `admin-users-create` invites a user from a dialog and lists the new row.
- `admin-users-detail` edits roles, resets a password, resends an invite, and disables a user.
- `admin-clients-create` registers a public or confidential client and its redirect URIs.
- `admin-client-secret` shows a confidential client secret once and can rotate it.
- `admin-roles` creates a role; `admin-groups` creates a group and adds members.
- `admin-sessions` revokes an active session.
- `admin-audit` filters immutable audit events.
- `admin-settings` edits password policy, session policy, and SMTP.

## How to get to it (user POV)

- Sign in as `admin@example.com`; the console lives at `/admin`.
- Sections: `/admin/users`, `/admin/clients`, `/admin/roles`, `/admin/groups`,
  `/admin/sessions`, `/admin/audit`, `/admin/settings`.
- A non-admin is redirected from `/admin` to `/account`.

## Driving it with Playwright

Preconditions:

- Stack healthy at `http://localhost:8093`.
- `web/e2e/.auth/admin.json` and `admin-totp.json` exist; the `setup` project
  writes both before any spec runs.

- **Admin gate.** Run `cd web && E2E_BASE_URL=http://localhost:8093 pnpm exec playwright test e2e/admin-mfa.spec.ts --trace on`. The spec signs in, completes the TOTP challenge, lands on `/admin`, and finds heading `Admin console` and link `Users`.
- **Create user.** Run `... pnpm exec playwright test e2e/admin-users.spec.ts`. From `/admin/users`, `Create user` opens a dialog headed `Create a user`; fill `Email` and `Full name`, choose `Create user`; the dialog shows `User created`; after `Done` the table holds the new email.
- **Create client.** Run `... pnpm exec playwright test e2e/admin-clients.spec.ts`. From `/admin/clients`, `Create client` opens `Create a client`; fill `Name` and `Redirect URIs`; the dialog closes and the row appears.
- **Confidential secret.** In a temporary spec, create a client and set `Type` to `Confidential`; the `AdminSecretDialog` shows the secret once with a copy control. Reload and confirm the secret is no longer displayed.
- **Revoke a session.** From `/admin/sessions`, choose the revoke action on a row; the row disappears and the matching `session` row gets `revoked_at`.
- **Audit trail.** Open `/admin/audit`; the user creation from the previous step appears as an event with the admin as actor.
- **Proof.** Screenshot the users table containing the new row, and read the row back with `select email from "user" where email = '<created email>';`.

## Gotchas

- A platform-admin without confirmed TOTP gets a restricted session: it can read
  itself, enroll, and log out, but every admin route returns `403` until `acr` is `mfa`.
- The admin TOTP secret lives in `web/e2e/.auth/admin-totp.json` and must match the
  database. A fresh stack always has a fresh database, so setup re-enrolls.
- The admin user spec asserts on a table row; pagination and filters can hide a
  newly created row if a previous run left many rows. The stack is disposable, so
  relaunch instead of fighting filters.
- Client secrets and invite tokens are shown once. Capture them during the action
  or read the invite link from MailHog.
- Creating users, roles, and clients accepts an `Idempotency-Key` at the API level;
  the SPA sends one per dialog submission. Replaying a request without it creates
  a second row.
- Disabling the last platform-admin is refused; do not use the fixture admin to
  test last-admin protection unless you create a second admin first.
