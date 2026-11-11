# Email flows

Password reset, invitations, and email verification move through an outbox to
SMTP. In this stack MailHog catches every message, so the link a user receives is
observable without a real inbox.

## Sub-features

- `forgot-request` accepts an email and always answers `202`, whether or not the account exists.
- `reset-mail` delivers `Reset your password` with a `/reset-password?token=` link.
- `reset-consume` sets a new password, signs out every session, and rejects the token on reuse.
- `invite-mail` sends `You are invited to sign in` with an `/accept-invite?token=` link.
- `invite-accept` sets a password, verifies the email, and returns to sign-in with a notice.
- `verify-email` consumes a verification token and shows `Email verified`.

## How to get to it (user POV)

- `/forgot-password` for a reset, reached from the `Forgot password?` link on `/login`.
- The reset link opens `/reset-password?token=...`.
- The invitation link opens `/accept-invite?token=...`.
- A verification link opens `/verify-email?token=...`.
- MailHog's inbox is at `http://localhost:8025`.

## Driving it with Playwright

Preconditions:

- Stack healthy at `http://localhost:8093`.
- The member fixture exists and you know its current password.

- **Request a reset.** In a temporary spec, `page.goto('/forgot-password')`, fill `Email` with `user@example.com`, choose `Send reset link`; the alert `Check your email` appears. The API answers `202` even for an unknown address.
- **Read the link.** `curl -s http://localhost:8025/api/v2/messages` returns the newest message with subject `Reset your password`. Bodies are quoted-printable; decode with `python3 -c 'import quopri,sys;print(quopri.decodestring(sys.stdin.buffer.read()).decode())'` and take the `/reset-password?token=...` URL.
- **Consume the token.** Run a temporary spec with `RESET_URL` in the environment: `page.goto(process.env.RESET_URL!)`, fill `New password` and `Confirm new password` with `Verified-Passw0rd!42`, choose `Save password`; the alert `Password updated` appears. Choose `Sign in with the new password`, sign in, and land on `/account`.
- **Reuse the token.** Request the same reset URL again; the page reports the link is invalid or expired, and the API answers `400`.
- **Invite.** Run `cd web && E2E_BASE_URL=http://localhost:8093 pnpm exec playwright test e2e/admin-users.spec.ts`. Then read MailHog for the new address; the message subject is `You are invited to sign in`.
- **Accept the invite.** Open the invite link, fill `Password` and `Confirm password` with `Invited-Passw0rd!42`, choose `Activate account`; the browser lands on `/login?notice=invite` and shows `Your password is set. Sign in to continue.` Sign in with that password and reach `/account`.
- **Verify the email.** Open a `/verify-email?token=...` link when one is issued; the alert `Email verified` appears. Read back `select email_verified_at from "user" where email = '<address>';` and require a non-null value.
- **Proof.** Keep the MailHog message (recipient, subject, link), the success screen, and the database row.

## Gotchas

- `POST /api/v1/auth/forgot-password` always answers `202`. A `202` is not proof that an account exists or that mail was sent; read MailHog.
- Tokens are single-use. A second use of the same link answers `400`; do not reuse a link across recipes.
- The password policy rejects a new password that contains the account's email or
  name. `E2E-User-Passw0rd!` was rejected as a new password for the member fixture
  during setup; use a distinct value.
- Resetting a password revokes every session, including the one you captured.
- MailHog's inbox is shared across stacks on this machine. Filter by recipient and
  read newest first; older messages from a previous run can match the same subject.
- Message bodies are quoted-printable, so links contain `=3D` for `=`. Decode
  before navigating.
- The API needs the double-submit CSRF token and an `Origin` header; the SPA sends
  both. Without `Origin: http://localhost:8093` the request answers `403`.
- No UI button sends a verification email on its own; the link comes from backend
  flows, and accepting an invitation also marks the email verified.
