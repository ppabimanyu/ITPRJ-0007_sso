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
`request_id`, `trace_id`, and `span_id`, plus `user_id` after authentication
lands. Email, tokens, passwords, cookies, and bodies are redacted.

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
