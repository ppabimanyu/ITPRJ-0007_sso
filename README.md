<img src="assets/icon.svg" width="64" height="64" alt="">

# ITPRJ-0007_sso

Self-hosted SSO service. M1 ships the Go backend, the operations foundation, and
a Vue 3 SPA shell that proves the frontend stack against the live API. Login
arrives in M2 and the admin console in M4, so this build exposes health,
readiness, version, a home status page, and an `/admin` placeholder.

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

Both commands converge on the same state when rerun. `rotate-key` and
`rotate-master-key` exit non-zero until M3.

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

MFA endpoints return 501 until M4.

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
