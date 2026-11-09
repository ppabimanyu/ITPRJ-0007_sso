<img src="assets/icon.svg" width="64" height="64" alt="">

# ITPRJ-0007_sso

Self-hosted SSO service. The M1 milestone ships the Go backend and operations
foundation. Login, OIDC, and the Vue console arrive in later milestones, so this
build intentionally exposes only health, readiness, and version endpoints.

## Requirements

- Go 1.27
- Docker with Compose v2
- `golangci-lint` v2 for `make lint`

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

## Endpoints

| Path | Purpose |
|---|---|
| `GET /healthz` | Process liveness |
| `GET /readyz` | Postgres, Redis, and migration version |
| `GET /api/v1/version` | Build identity |
| `GET /` | Embedded SPA placeholder |

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
| `make web-build` | Build the SPA once `web/package.json` exists |
| `make clean` | Remove build output |

## Checks

```sh
go build ./...
go vet ./...
go test ./...
golangci-lint run
docker compose config
```

CI runs vet, golangci-lint, govulncheck, and tests. The frontend job is added by
the SPA owner.
