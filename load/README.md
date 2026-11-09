# Load tests

Targets from the PRD:

- p95 `POST /realms/{realm}/token` below 150 ms.
- p95 login until the session exists below 500 ms at normal load.
- Peak shape up to 50 login attempts per second.

The Argon2id profile uses 64 MiB per hash. `ARGON2_CONCURRENCY` bounds how many
hashes run at once, so a saturated queue adds latency instead of exhausting
memory.

## Run

Start a throwaway stack with high rate limits. The login limit is per IP, and a
load run comes from one address.

```sh
export E2E_PORT=8092
export E2E_DB=load_$(date +%s)
export AUTH_LOGIN_IP_LIMIT=1000000
export AUTH_IP_LIMIT=1000000
export API_IP_LIMIT=1000000
export OIDC_IP_LIMIT=1000000
export TOKEN_CLIENT_LIMIT=1000000
export ARGON2_CONCURRENCY=2
bash scripts/e2e-stack.sh >/tmp/sso-load-stack.log 2>&1 &
```

The seed prints the confidential client secret once. Read it from the log, then
run either scenario.

```sh
export BASE_URL=http://localhost:8092
export CLIENT_SECRET="$(sed -n 's/^seed: dev-only client secret (shown once): //p' /tmp/sso-load-stack.log | tail -1)"

k6 run load/token.js
k6 run load/login.js
```

Both scripts enforce their p95 threshold and fail when it is crossed. Record
the summary in the milestone notes and name the limiting resource. On a laptop,
the CPU running Argon2 is usually the first limit for login; token issuance
measures signing and database reads.

## Baseline, 10 November 2026

Local Apple Silicon Mac, Postgres and Redis in Docker, `ARGON2_CONCURRENCY=2`,
one run per script.

- `token.js`, 50 iterations per second for 30 s: 1500 requests, p95 17.4 ms,
  0 failures. Target is 150 ms.
- `login.js`, 10 VUs for 35 s: 251 logins at about 7.1 per second, p95 209 ms,
  0 failures. Target is 500 ms.

The login run did not reach 50 logins per second. The observed queueing and the
fixed concurrency point to Argon2id at 64 MiB as the limiter, inferred from the
guard rather than a profile. Raise `ARGON2_CONCURRENCY` on a host with more
cores, or add login workers, before claiming the 50 per second target. The token
path was not the limiter in this run.
