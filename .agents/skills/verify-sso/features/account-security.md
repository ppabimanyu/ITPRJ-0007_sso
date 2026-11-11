# Account and security

The account page lets a signed-in user edit their profile, change their password,
review and revoke sessions, and manage authenticator-app two-step verification
with one-time recovery codes.

## Sub-features

- `profile-save` updates full name and language and shows `Profile saved.`
- `password-change` requires the current password, then signs out other sessions.
- `session-list` shows the current session and any others with device, IP, and expiry.
- `session-revoke` removes one other session and its row.
- `totp-enroll` starts enrollment, confirms a six-digit code, and shows ten recovery codes once.
- `totp-disable` removes the authenticator and its recovery codes after a confirm dialog.
- `recovery-regenerate` replaces the recovery codes exactly once.

## How to get to it (user POV)

- Open `/account` after signing in.
- Use the tabs `Profile`, `Password`, `MFA`, and `Sessions` on that page.
- The admin lands on `/mfa/enroll` first when TOTP is not enrolled; the account
  page is reachable afterwards.
- `Enable two-step verification` opens `/mfa/enroll`.

## Driving it with Playwright

Preconditions:

- Stack healthy at `http://localhost:8093`.
- Signed in as the member fixture, either with `test.use({ storageState: userStatePath })`
  or by calling `signInAs(page, userIdentity.email, userIdentity.password)`.

- **Save profile.** Open `/account`, tab `Profile`, fill `Full name` with `Sample User`, choose `Save profile`. The alert `Profile saved.` appears.
- **Password change.** Tab `Password`, fill `Current password` with the fixture password, `New password` and `Confirm new password` with `Verified-Passw0rd!42`, choose `Change password`. The alert `Password updated. Other sessions were signed out.` appears; the current page stays signed in.
- **Session list.** Tab `Sessions`. Headings `This session` and `Other sessions` appear; the current device is labelled `This device`.
- **Revoke another session.** Sign in from a second browser context, return to the first, tab `Sessions`, choose `Revoke` on the other session. The row disappears; the revoked context gets `401` from `GET /api/v1/auth/session`.
- **TOTP enroll.** Tab `MFA`, choose `Enable two-step verification`. `/mfa/enroll` shows the setup key at `#mfa-enroll-secret`; fill `Authentication code` with `totpCode(secret, await waitForFreshStep())`; choose `Confirm and enable`. The heading `Save your recovery codes` appears; choose `I saved these codes` and land back on the account page.
- **Disable TOTP.** Tab `MFA`, choose `Disable two-step verification`, then `Disable` in the confirm dialog. The status text becomes `Off` and the next login no longer asks for a code.
- **Proof.** Screenshot the MFA panel after enrollment and read `select count(*) from mfa_recovery_code` for the user from Postgres.

## Gotchas

- The password policy rejects a new password that contains the account's email or
  name. `E2E-User-Passw0rd!` was rejected as a new password during setup; use
  `Verified-Passw0rd!42` or similar.
- A TOTP time step is single-use. Reusing a code fails; wait for a fresh step with
  `e2e/totp.ts`.
- Recovery codes are displayed once. A reload after enrollment loses them.
- Changing the password keeps the current session and revokes the others; do not
  assert that every session is gone.
- Disabling TOTP is confirmed in a dialog (`Disable two-step verification?`), not
  on the first click.
- The admin cannot edit the account page until forced enrollment finishes; every
  admin route returns `403` before that.
