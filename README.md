<img src="assets/icon.svg" width="64" height="64" alt="">

# ITPRJ-0007_sso

Self-hosted SSO service. M1 ships the Go backend, the operations foundation, and
a Vue 3 SPA shell that proves the frontend stack against the live API. M2 adds
password login, sessions, invite, reset, and the email outbox. M3 adds the OIDC
provider, so a relying party can run a PKCE authorization code flow, refresh
tokens, read userinfo, and log out. M4 adds TOTP MFA, forced admin enrollment,
and the admin user API. The admin console arrives separately.

The SPA login and consent screens for the OIDC redirect are owned separately
(M3-14). The backend flow is complete and testable with curl.

## Requirements

- Go 1.27
- Docker with Compose v2
- `golangci-lint` v2 for `make lint`
- Node.js 22 or newer for SPA development and `make web-build`

The Go build and the shipped binary do not need Node. A fresh clone embeds a
small built-in "UI not built" page until you build the SPA.

## First run

```sh
cp .env.example .env
make dev
```

`make dev` starts Postgres, Redis, and MailHog, applies migrations, then builds
and starts the app and Caddy. The app listens on `http://localhost:8080`.
MailHog's inbox is at `http://localhost:8025`.

If a host port is taken, set `POSTGRES_PORT`, `REDIS_PORT`, `APP_PORT`, or
`CADDY_PORT` in `.env`, and update `DATABASE_URL` and `REDIS_URL` to match.
The committed defaults are the standard ports.

## Database setup

Migrations run separately from the app. With `.env` in place:

```sh
go run ./cmd/migration up
go run ./cmd/migration down   # roll back one migration
```

Create the default realm, `platform-admin` role, admin user, and an encrypted
RSA-3072 signing key. There is no default password:

```sh
printf '%s\n' 'Dev-Only-Passw0rd!' \
  | go run ./cmd/migration bootstrap --email admin@example.com --password-stdin
```

Seed local fixtures: a sample verified user, a `member` role, a public SPA
client (`sample-spa`), and a confidential backend client (`sample-backend`).
The generated dev-only client secret is printed once:

```sh
printf '%s\n' 'Dev-Only-Passw0rd!' | go run ./cmd/migration seed --password-stdin
```

Both commands converge on the same state when rerun. Rotate the realm signing
key with `go run ./cmd/migration rotate-key`. The command creates a new active
RSA-3072 key and retires the old one after 24 hours, so tokens signed before the
rotation keep verifying inside that window. `rotate-master-key` remains
reserved for a later milestone.

## Auth API (M2)

Login, session, and account endpoints live under `/api/v1`. The SPA is not part
of this milestone; the API is complete and testable with curl.

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/v1/auth/csrf` | Issue the pre-session CSRF token and cookie |
| `POST` | `/api/v1/auth/login` | Email and password login |
| `GET` | `/api/v1/auth/session` | Current user, roles, and expiry |
| `POST` | `/api/v1/auth/logout` | Revoke the current session |
| `POST` | `/api/v1/auth/forgot-password` | Queue a reset email, always 202 |
| `POST` | `/api/v1/auth/reset-password` | Consume the reset token and revoke sessions |
| `POST` | `/api/v1/auth/verify-email` | Consume a verification token |
| `POST` | `/api/v1/auth/accept-invite` | Set a password and verify the email |
| `GET`/`PATCH` | `/api/v1/me` | Read or change the profile |
| `POST` | `/api/v1/me/password` | Change the password, revoke other sessions |
| `GET` | `/api/v1/me/sessions` | List active sessions |
| `DELETE` | `/api/v1/me/sessions/{id}` | Revoke one owned session |
| `POST` | `/api/v1/auth/mfa/verify` | Finish login with a TOTP or recovery code |
| `POST` | `/api/v1/auth/mfa/enroll` | Start or confirm forced admin TOTP enrollment |
| `POST` | `/api/v1/me/mfa/totp` | Start or confirm TOTP enrollment |
| `DELETE` | `/api/v1/me/mfa/totp` | Remove TOTP and its recovery codes |
| `POST` | `/api/v1/me/mfa/recovery-codes` | Regenerate recovery codes once |

### MFA

A user with a confirmed TOTP secret gets no session from `POST
/api/v1/auth/login`. The response is `{"status":"mfa_required"}` with an
`mfa_token`. Complete it at `/api/v1/auth/mfa/verify` with either `code` or
`recovery_code`. The challenge lives in Redis for five minutes and is destroyed
after five failures. A successful challenge creates the session with
`amr=["pwd","otp"]` and `acr="urn:itprj:sso:acr:mfa"`.

A `platform-admin` without a confirmed TOTP gets a restricted session and
`{"status":"mfa_enrollment_required"}`. That session may read its own session,
enroll TOTP, and log out. Every admin route returns 403 until enrollment
completes and the session's `acr` becomes `mfa`.

`action=start` returns the secret and `otpauth://` URI once. Scan it, then send
`action=confirm` with the current six-digit code. Confirmation returns ten
recovery codes once; the database stores only SHA-256 hashes. TOTP uses a
30-second step with a ±1 window and rejects a reused time step.

### Admin users

All `/api/v1/users` routes require a `platform-admin` session with a completed
MFA challenge. Errors use `application/problem+json`.

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/v1/users` | Keyset list with `after`, `limit`, `q`, and `status` |
| `POST` | `/api/v1/users` | Create an invite-only user, optional `Idempotency-Key` |
| `GET` | `/api/v1/users/{id}` | User with roles, groups, MFA status, and session count |
| `PATCH` | `/api/v1/users/{id}` | Update profile or disable a user |
| `POST` | `/api/v1/users/{id}/reset-password` | Queue a reset email, 202 |
| `POST` | `/api/v1/users/{id}/invite` | Resend the invite, 202 |
| `POST` | `/api/v1/users/{id}/roles` | Replace the user's roles |

Disabling a user revokes every session and refresh token. The last active
`platform-admin` in a realm cannot be disabled or lose the role. A replayed
`Idempotency-Key` returns the stored 201 response; reusing it with a different
body returns `409 idempotency_mismatch`.

### Admin resources

The rest of the admin surface uses the same `platform-admin` plus completed
MFA session. Writes need the session CSRF token, and create endpoints honor
`Idempotency-Key`.

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/v1/clients` | Keyset list with `after` and `limit` |
| `POST` | `/api/v1/clients` | Create a client, optional `Idempotency-Key` |
| `GET` | `/api/v1/clients/{id}` | Client detail without its secret |
| `PATCH` | `/api/v1/clients/{id}` | Replace URI lists, origins, grants, scopes, TTLs, or disable |
| `DELETE` | `/api/v1/clients/{id}` | Delete only without an active session or consent |
| `POST` | `/api/v1/clients/{id}/rotate-secret` | Return a new confidential secret once |
| `GET` | `/api/v1/roles` | List realm roles |
| `POST` | `/api/v1/roles` | Create a role, optional `Idempotency-Key` |
| `GET` | `/api/v1/groups` | List groups with member counts |
| `POST` | `/api/v1/groups` | Create a group, optional `Idempotency-Key` |
| `GET` | `/api/v1/groups/{id}` | Group detail |
| `PATCH` | `/api/v1/groups/{id}` | Rename or change the description |
| `DELETE` | `/api/v1/groups/{id}` | Delete a group and its membership |
| `GET` | `/api/v1/groups/{id}/members` | List members |
| `POST` | `/api/v1/groups/{id}/members` | Add users by `user_ids` |
| `DELETE` | `/api/v1/groups/{id}/members/{user_id}` | Remove a member |
| `GET` | `/api/v1/sessions` | List sessions with `user_id`, `status`, `after`, and `limit` |
| `DELETE` | `/api/v1/sessions/{id}` | Revoke a session and its refresh family |
| `GET` | `/api/v1/audit` | Filter by `from`, `to`, `type`, `actor_id`, `after`, and `limit` |
| `GET` | `/api/v1/realm/settings` | Read SMTP, password, and session policy |
| `PATCH` | `/api/v1/realm/settings` | Update the same settings |

Confidential client secrets are generated on create and returned once. Rotation
returns the new value once and revokes the previous hashes. Redirect URIs match
exactly and must use HTTPS, except loopback HTTP in dev; wildcards are rejected.
A backchannel URI must not resolve to a literal private or loopback address
outside dev. Disabling a client revokes its refresh tokens. Sessions are
user-wide rather than client-scoped, so a session shared with another client
stays alive; delete stays blocked while a client has a consent row or a live
refresh token.

Groups are flat. Membership changes affect the next token, never one already
issued. Audit metadata never contains passwords, tokens, secrets, or email
addresses; the viewer shows user IDs and joins email separately when needed.

### Realm SMTP

`realm.settings` stores the password and session policy plus per-realm SMTP.
SMTP passwords are encrypted with the active master key and are write-only: GET
returns `password_set` and never the value. An empty `smtp.host` or
`smtp.from_address`, or a realm with no settings, makes the outbox worker use
the environment `SMTP_*` sender. Development can set MailHog explicitly:

```sh
curl -s -b cookies.txt -X PATCH \
  -H "X-CSRF-Token: $SESSION_CSRF" -H 'Content-Type: application/json' \
  -d '{"smtp":{"host":"localhost","port":1025,"from_address":"no-reply@example.com","use_tls":false,"password":"dev-only-smtp-password"}}' \
  http://localhost:8080/api/v1/realm/settings
```

### Cookies and CSRF

Production uses the `__Host-sso_session` cookie with `HttpOnly`, `Secure`, and
`SameSite=Lax`. Development over HTTP uses `sso_session` without `Secure`.
`sso_csrf` is readable by JavaScript and carries the double-submit token.

Pre-session endpoints need a token from `GET /api/v1/auth/csrf`. The raw token
is stored only as a SHA-256 digest in Redis for 15 minutes. Send the same value
in `X-CSRF-Token` and as the `sso_csrf` cookie, and send an `Origin` header that
appears in `ALLOWED_ORIGINS`. Successful login replaces the pre-session token
with a session-bound token whose digest is stored on the session row. Logout
and every `/api/v1/me` write check that session-bound token.

```sh
curl -s -c cookies.txt -H 'Origin: http://localhost:5173' \
  http://localhost:8080/api/v1/auth/csrf -o csrf.json
CSRF=$(sed -E 's/.*"csrf_token":"([^"]+)".*/\1/' csrf.json)

curl -s -b cookies.txt -c cookies.txt \
  -H "X-CSRF-Token: $CSRF" -H 'Origin: http://localhost:5173' \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"your-dev-password"}' \
  http://localhost:8080/api/v1/auth/login

curl -s -b cookies.txt http://localhost:8080/api/v1/auth/session
```

Password reset and verification links point at `EXTERNAL_URL`. The worker polls
the outbox every second and delivers through `SMTP_HOST:SMTP_PORT` (MailHog at
`http://localhost:8025` in Compose). `SMTP_TLS=true` requires STARTTLS.

## OIDC provider (M3)

The provider is mounted under `/realms/{realm}`. The seeded realm is `default`.

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/realms/{realm}/.well-known/openid-configuration` | Discovery document |
| `GET` | `/realms/{realm}/jwks` | Public signing keys |
| `GET` | `/realms/{realm}/authorize` | Authorization endpoint |
| `POST` | `/realms/{realm}/token` | Authorization code, refresh, and client credentials |
| `GET`/`POST` | `/realms/{realm}/userinfo` | User claims for a bearer access token |
| `POST` | `/realms/{realm}/revoke` | RFC 7009 revocation |
| `POST` | `/realms/{realm}/introspect` | RFC 7662 introspection |
| `GET`/`POST` | `/realms/{realm}/logout` | RP-initiated logout |
| `POST` | `/realms/{realm}/logout/backchannel` | Inbound signed logout token |

Discovery is the source of truth for the issuer and endpoint URLs:

```sh
curl -s http://localhost:8080/realms/default/.well-known/openid-configuration | jq
curl -s http://localhost:8080/realms/default/jwks | jq '.keys[] | {kid, kty, alg, use}'
```

### Relying party setup

Register these values for each client. The admin API for this arrives in M4, so
local fixtures come from `seed`.

- A public client has `type=public`, no secret, and must send PKCE S256.
- A confidential client has `type=confidential` and authenticates with
  `client_secret_basic` or `client_secret_post`.
- Grant types are `authorization_code`, `refresh_token`, and
  `client_credentials`.
- Allowed scopes are `openid`, `profile`, `email`, `offline_access`, `roles`,
  and `groups`. `offline_access` is the only way to receive a refresh token.
- Redirect URIs match exactly, including scheme, host, port, path, and query.
  No wildcard or prefix match exists.
- Post-logout redirect URIs are registered with the same exact-match rule.
- CORS origins are registered per client for `/token`, `/userinfo`, `/revoke`,
  and `/introspect`. Preflight requests are matched against the realm's
  registered origins. An authenticated request is matched against the client's
  own origins.

The seeded clients are ready for local RP tests:

| Client | Type | Redirect | CORS origin |
|---|---|---|---|
| `sample-spa` | public | `http://localhost:5173/callback` | `http://localhost:5173` |
| `sample-backend` | confidential | `http://localhost:8080/callback` | `http://localhost:8080` |

`sample-backend` prints its dev-only secret once during `seed`. Rerunning seed
does not rotate it.

### Authorization code flow

1. The RP sends the browser to `/authorize` with `response_type=code`,
   `client_id`, `redirect_uri`, `scope`, `state`, `nonce`, `code_challenge`, and
   `code_challenge_method=S256`.
2. The provider stores the request in Redis for 10 minutes and sets an HttpOnly
   `sso_authreq` cookie bound to the request. It redirects to
   `/login?request_id=...`, or `/consent?request_id=...` for a third-party
   client without a matching consent.
3. The SPA calls `GET /api/v1/auth/authorize/request?request_id=...`, shows the
   client and scopes, completes login, then calls
   `POST /api/v1/auth/authorize/complete` with the session CSRF token. A
   third-party flow calls `POST /api/v1/auth/consent` with the approved scopes.
   Both return `{ "redirect_to": "..." }`.
4. The browser follows `redirect_to` to the registered callback with `code` and
   `state`.
5. The RP exchanges the code at `/token` with the same `redirect_uri` and
   `code_verifier`. Codes are single use and expire after 60 seconds.

The complete browser flow needs the SPA screens. The integration test drives the
same requests with a cookie jar and a fake RP.

```sh
# Discovery, as seen by an RP.
curl -s http://localhost:8080/realms/default/.well-known/openid-configuration

# Confidential client credentials, no refresh or ID token is returned.
SECRET='the value printed by seed'
curl -s -u "sample-backend:$SECRET" \
  -H 'Origin: http://localhost:8080' \
  -d 'grant_type=client_credentials&scope=openid%20profile' \
  http://localhost:8080/realms/default/token
```

Access tokens are RS256 JWTs valid for 15 minutes. ID tokens are RS256 JWTs
valid for 5 minutes. Refresh tokens are opaque, hashed at rest, rotate on every
use, and revoke their whole family when an already-used token is presented.
Introspection checks the session and user state, not only the signature.

## Run locally without containers

```sh
make build
./bin/server
```

The server reads `.env` when present and does not override exported variables.
`APP_ENV`, `EXTERNAL_URL`, `DATABASE_URL`, `REDIS_URL`, `SSO_MASTER_KEYS`, and
`SSO_MASTER_KEY_ID` are required. Dev-only master keys belong in `.env` only.

## SPA development

The SPA lives in `web/` and uses Vite, Vue 3, TypeScript, vue-router, TanStack
Query, vue-i18n (`en` and `id`, browser default), and Tailwind CSS v3 with
shadcn-vue conventions.

```sh
cd web
npm install      # install dependencies from the committed lockfile
npm run dev      # http://localhost:5173, proxies /api and /healthz to :8080
npm run lint
npm run typecheck
npm run build    # writes web/dist
```

`VITE_API_BASE_URL` defaults to empty, so the SPA calls the API on its own
origin. Set it to an absolute origin when the SPA is served from a different
host than the API.

To serve the built SPA from the Go binary:

```sh
make web-build
make build
```

`make web-build` runs `npm ci`, builds `web/dist`, and replaces the contents of
`internal/webui/dist`. The Go build embeds that directory. Hashed assets are
not tracked, so only `internal/webui/dist/.gitkeep` lives in the repository.

## Ports

| Service | URL |
|---|---|
| App | `http://localhost:8080` |
| Vite dev server | `http://localhost:5173` |
| Caddy | `http://localhost:8088` |
| MailHog UI | `http://localhost:8025` |
| PostgreSQL | `localhost:5432` |
| Redis | `localhost:6379` |

## Endpoints

| Path | Purpose |
|---|---|
| `GET /healthz` | Process liveness |
| `GET /readyz` | Postgres, Redis, and migration version |
| `GET /api/v1/version` | Build identity |
| `GET /realms/{realm}/.well-known/openid-configuration` | OIDC discovery |
| `GET /realms/{realm}/jwks` | OIDC signing keys |
| `GET /realms/{realm}/authorize` | OIDC authorization |
| `POST /realms/{realm}/token` | OIDC token endpoint |
| `GET /` | SPA home, or the built-in not-built page |
| `GET /admin` | SPA admin placeholder; the console lands in M4 |

Every response carries `X-Request-ID` and a W3C `traceparent`. JSON logs include
`request_id`, `trace_id`, and `span_id`, plus `user_id` after session
authentication. Email, tokens, passwords, cookies, and bodies are redacted.

## Make targets

| Target | Action |
|---|---|
| `make dev` | Start the full Compose stack |
| `make build` | Build `bin/server` and `bin/migration` |
| `make test` | `go test ./...` |
| `make lint` | `golangci-lint run` |
| `make migrate` | Apply migrations |
| `make seed` | Seed fixtures (password on stdin) |
| `make bootstrap` | Create the first admin (`EMAIL=...`) |
| `make web-build` | Build the SPA and embed it in `internal/webui/dist` |
| `make clean` | Remove Go, web, and embedded build output |

## Checks

```sh
go build ./...
go vet ./...
go test ./...
golangci-lint run

cd web
npm ci
npm run lint
npm run typecheck
npm run build

docker compose config
```

CI runs a Web job for install, lint, typecheck, and build. The Go job rebuilds
the SPA, copies it into `internal/webui/dist` before `go build` and `go test`,
then runs vet, golangci-lint, and govulncheck.
