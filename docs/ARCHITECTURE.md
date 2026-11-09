# ARCHITECTURE — SSO Service (ITPRJ-0007)

| | |
|---|---|
| Status | Draft v1.0 |
| Tanggal | 9 November 2026 |
| Dokumen acuan | `docs/PRD.md` v1.1 |
| Kontrak API | `api/openapi.yaml` |

Dokumen ini menjelaskan bagaimana keputusan di PRD diterjemahkan menjadi struktur kode, alur runtime, aturan transaksi, dan pengamanan. Jika terjadi konflik, PRD menang; dokumen ini harus diperbarui agar tetap konsisten.

---

## 1. Prinsip Arsitektur

1. **Logika bisnis di handler.** `internal/domain` hanya berisi konstanta, DTO, model, dan interface. Tidak ada service layer.
2. **Transaksi di repository.** Handler tidak membuka transaksi dan tidak menyentuh `pgx`. Operasi tulis atomik diekspos sebagai satu method repository bernama bisnis.
3. **Postgres sumber kebenaran.** Sesi, token, consent, MFA, audit, dan outbox hidup di Postgres. Redis hanya cache dan state ephemeral.
4. **Protokol di tepi, kebijakan di kita.** Handler OIDC memakai `go-jose/v4` untuk JWT dan JWKS; konstanta wire diambil dari `zitadel/oidc/v3`. `op.Storage` tidak dipakai karena mengasumsikan OP memiliki redirect login/consent, sedangkan UI adalah SPA. Conformance suite menjadi gate rilis.
5. **Satu binary, satu image.** SPA di-embed via `embed.FS`; server dan worker berjalan di proses yang sama pada v1.
6. **Realm adalah batas keamanan.** Semua query dan method repository menerima `realm_id`; tidak ada lookup lintas realm.

## 2. Diagram Komponen

```
 ┌─────────────┐        ┌─────────────┐
 │  RP SPA     │        │ RP Backend  │
 │ (public)    │        │(confidential)│
 └──────┬──────┘        └──────┬──────┘
        │ browser redirects     │ /token, /introspect
        ▼                       ▼
 ┌─────────────────────────────────────────┐
 │              Caddy (TLS)                │
 └──────────────────┬──────────────────────┘
                    ▼
 ┌─────────────────────────────────────────────────────────────┐
 │                        Go binary                            │
 │  ┌──────────────────────┐   ┌────────────────────────────┐  │
 │  │ internal/httphandler │   │ internal/workerhandler     │  │
 │  │  REST /api/v1        │   │  outbox email, cleanup,    │  │
 │  │  OIDC /realms/{r}/*  │   │  backchannel logout        │  │
 │  └──────────┬───────────┘   └────────────┬───────────────┘  │
 │             │  (logika bisnis di sini)   │                  │
 │             ▼                            ▼                  │
 │  ┌──────────────────────────────────────────────────────┐   │
 │  │ interface: internal/domain/port_repo_*.go            │   │
 │  └──────────┬───────────────────────────────────────────┘   │
 │             ▼                                               │
 │  ┌──────────────────────────────────────────────────────┐   │
 │  │ internal/repository                                  │   │
 │  │  repo_pg_*, repo_redis_*, repo_{http,grpc,kafka,smtp}_* │   │
 │  └───────┬───────────────────────┬──────────────────────┘   │
 └──────────┼───────────────────────┼──────────────────────────┘
            ▼                       ▼
     ┌────────────┐          ┌────────────┐        ┌───────────┐
     │ PostgreSQL │          │   Redis    │        │   SMTP    │
     └────────────┘          └────────────┘        └───────────┘
```

Kotak `internal/grpchandler`, `internal/eventhandler`, `repo_grpc_*`, dan `repo_kafka_*` disiapkan sebagai kontrak kosong dan belum diaktifkan di v1.

## 3. Struktur Paket & Tanggung Jawab

```
cmd/server/main.go              # wiring, HTTP server, worker, graceful shutdown
cmd/migration/main.go           # up, down, seed, bootstrap, rotate-key, rotate-master-key
internal/domain/
  const_realm.go                # konstanta realm, scope, acr, amr
  const_error.go                # kode error internal
  dto_auth.go                   # request/response handler
  dto_admin.go
  dto_oidc.go
  model_user.go                 # representasi data untuk repository/DTO
  model_session.go
  port_repo_user.go             # interface repository
  port_repo_session.go
  port_repo_oidc.go
  port_ext_mail.go
internal/httphandler/
  httphandler_router.go         # mount route, middleware
  httphandler_auth.go           # login, MFA, consent, invite, reset
  httphandler_oidc.go           # authorize/token/userinfo/logout/discovery
  httphandler_admin_user.go
  httphandler_admin_client.go
  httphandler_admin_audit.go
  httphandler_me.go
  httphandler_middleware.go     # request ID, CSRF, auth, realm, rate limit
internal/repository/           # FLAT: implementasi query DB + external service
  repo_pg_user.go              # query Postgres per entitas
  repo_pg_session.go
  repo_pg_client.go
  repo_pg_audit_outbox.go
  repo_pg_realm.go
  repo_pg_key.go
  repo_redis_session.go
  repo_redis_authcode.go
  repo_redis_ratelimit.go
  repo_oidc_storage.go         # implementasi op.Storage
  repo_smtp_mail.go            # external service SMTP
  repo_http_*.go               # external service REST (bila ada)
  repo_grpc_*.go               # external service gRPC (kontrak)
  repo_kafka_*.go              # external service Kafka (kontrak)
internal/infra/
  caching/                     # client Redis
  logger/                      # slog JSON + trace/span/user enrichment + redaksi
  tracing/                     # W3C trace/span context + helper; exporter OTel ditunda
  kafka/                       # producer/consumer client (kosong di v1)
  database/                    # pgx pool, runner goose, helper transaksi internal repo
  config/                      # env + YAML opsional
internal/pkg/                   # random, base64url, clock, password list, dll.
migrations/                     # goose SQL
api/openapi.yaml
web/
deploy/
```

### Aturan dependensi

| Paket | Boleh mengimpor |
|---|---|
| `domain` | stdlib saja |
| `repository` | `domain`, `infra`, library DB/Redis |
| `httphandler` | `domain`, `repository` (lewat interface), `infra/logger`, `infra/config` |
| `workerhandler` | `domain`, repository, `infra` |
| `infra` | stdlib + driver |
| `cmd` | semua paket untuk wiring |

`domain` tidak boleh mengimpor `repository` atau `infra`. Handler menerima interface dari `domain`, implementasi di-inject dari `cmd/server`.

### Batas infra vs repository vs handler

| Lapisan | Isi | Contoh |
|---|---|---|
| `infra` | koneksi/client low-level tanpa aturan bisnis | `caching/redis.go`, `logger/slog.go`, `kafka/producer.go` |
| `repository` | implementasi `port_repo_*`/`port_ext_*`: query database dan panggilan external service, **flat tanpa subfolder** | `repo_pg_user.go`, `repo_http_org.go`, `repo_grpc_org.go`, `repo_kafka_audit.go` |
| `httphandler` | orkestrasi bisnis, validasi DTO, pemanggilan satu atau beberapa method repository | `httphandler_auth.go` |

- Pengelompokan file repository hanya lewat prefix nama (`repo_pg_`, `repo_redis_`, `repo_http_`, `repo_grpc_`, `repo_kafka_`, `repo_smtp_`), bukan folder.
- Handler tidak mengimpor client infra secara langsung; satu-satunya pengecualian adalah `logger` dan `config` untuk kebutuhan request-scoped logging.
- Implementasi external service REST/gRPC/Kafka tetap berada di `internal/repository` karena mereka mengimplementasikan port `port_ext_*`, bukan menyediakan koneksi mentah.

### Tanpa service layer

Konsekuensi yang diterima:

- Handler mengorkestrasi: validasi DTO → panggil satu atau beberapa method repository → bentuk respons.
- Logika lintas endpoint diduplikasi atau dijadikan helper privat di paket `httphandler` (mis. `httphandler_helper_token.go`). Tidak ada paket "service" tersembunyi.
- Setiap operasi tulis yang harus atomik menjadi **satu** method repository. Contoh nama: `CreateSessionWithTokens`, `RotateRefreshToken`, `CompletePasswordReset`, `DisableUserAndRevokeSessions`, `CreateUserWithInvite`.
- Method repository menerima `context.Context` dan DTO domain; tidak menerima `http.Request`.
- Repository boleh memanggil repository lain di paket yang sama selama tetap dalam satu transaksi internal.

## 4. Middleware & Request Lifecycle

Urutan middleware global:

1. `RequestID` — generate/teruskan `X-Request-ID`, masukkan ke context dan log.
2. `Tracing` — parse `traceparent` (W3C) atau buat `trace_id`/`span_id` baru, simpan di context.
3. `RealIP` — hanya percaya `X-Forwarded-*` dari `TRUSTED_PROXY_CIDRS`.
4. `Recoverer` — panic → 500 problem+json, log stack tanpa body sensitif.
5. `SecurityHeaders` — CSP, HSTS (di produksi), `X-Content-Type-Options`, `Referrer-Policy: no-referrer`, `X-Frame-Options: DENY` untuk admin.
6. `BodyLimit` — 1 MB untuk JSON, 64 KB untuk form OIDC.
7. `Logger` — method, path (tanpa query sensitif), status, durasi, `request_id`, `trace_id`, `span_id`, `user_id` bila ada.
8. `RateLimit` — per route group, fail-closed untuk auth saat Redis mati.
9. `RealmResolver` — ambil `{realm}` dari path OIDC atau realm default untuk `/api/v1`.
10. `SessionAuth` — baca cookie, validasi sesi, isi context termasuk `user_id` (kecuali route publik).
11. `CSRF` — untuk route ber-cookie; bandingkan header dengan cookie dan HMAC sesi.

Route publik: `/api/v1/auth/login`, `/forgot-password`, `/reset-password`, `/verify-email`, `/accept-invite`, `/realms/*`.

### Korelasi log

- `logger.FromContext(ctx)` mengembalikan `*slog.Logger` yang otomatis menambahkan `trace_id`, `span_id`, dan `user_id` (jika ada).
- `trace_id`/`span_id` berasal dari `traceparent` masuk; jika tidak valid/tidak ada, middleware membuat 16-byte trace ID dan 8-byte span ID acak.
- `user_id` diisi setelah `SessionAuth`; handler yang berjalan sebelum autentikasi tidak mencetak field tersebut.
- `user_id` adalah UUID internal, bukan email. Log tidak boleh memuat email, token, password, atau isi request sensitif.
- Span anak untuk query/DB boleh dibuat di masa depan; logger tetap memakai span ID request sampai itu ada.

## 5. Alur Runtime

### 5.1 Authorize → Login → Code

```
RP                     SPA/IdP                         Handler/Redis/PG
│ GET /authorize?...     │                                  │
├───────────────────────►│                                  │
│                        │ validate client/redirect/PKCE     │
│                        │ simpan auth request (Redis 10m)   │
│                        │ set cookie sso_authreq            │
│                        │ 302 /login?request_id=...         │
│                        ├─────────────────────────────────►│
│                        │ GET /api/v1/auth/authorize/request│
│                        │◄─────────────────────────────────┤
│                        │ tampilkan client name + scope     │
│                        │ POST /api/v1/auth/login           │
│                        │  (email, password, request_id)    │
│                        ├─────────────────────────────────►│
│                        │ rate limit, lockout, Argon2       │
│                        │ jika MFA: mfa_required + token     │
│                        │ jika tidak: buat sesi + cookie    │
│                        │◄─────────────────────────────────┤
│                        │ POST /authorize/complete          │
│                        ├─────────────────────────────────►│
│                        │ cek sesi + cookie authreq         │
│                        │ buat auth code (Redis 60s)        │
│                        │◄── { redirect_to: RP?code=... }   │
│ 302 code               │                                  │
│◄───────────────────────┤                                  │
```

Aturan pengamanan:

- `request_id` adalah 128-bit acak, disimpan di Redis dengan TTL 10 menit, dan **diikat ke cookie** `sso_authreq` (HttpOnly, SameSite=Lax, Secure). Completion tanpa cookie yang cocok ditolak.
- Auth request menyimpan `client_id`, `redirect_uri`, `scope`, `state`, `nonce`, `code_challenge`, `prompt`, `max_age`, dan `session_id` setelah login.
- `prompt=none`: jika tidak ada sesi valid → redirect error `login_required`; jika butuh MFA/consent → `interaction_required`.
- `max_age`: bandingkan `auth_time` sesi; jika lebih tua → reauth.
- Auth code single-use; pengambilan memakai operasi atomik Redis (`GETDEL`).

### 5.2 Login & Sesi

- Validasi email case-insensitive, password Argon2id.
- Lockout disimpan di Postgres (`user_credential.failed_attempts`, `locked_until`) agar selamat dari flush Redis; rate limit per IP di Redis.
- Jika MFA aktif: buat `mfa_token` acak di Redis (TTL 5 menit) berisi `user_id`, `realm_id`, `request_id`, dan counter percobaan. Tidak ada sesi sebelum MFA selesai.
- Setelah sukses: `CreateSession` di repository (satu transaksi: sesi + audit + outbox jika perlu), set cookie sesi, rotasi CSRF token.
- Cookie produksi: `__Host-sso_session`; dev: `sso_session` (tanpa prefix) karena HTTP.
- Idle timeout diperbarui maksimal sekali per menit untuk mengurangi write.
- Kebijakan idle admin (15 menit) dievaluasi dari role saat request; role dicache singkat (≤30 detik).

### 5.3 Token Exchange

`POST /realms/{realm}/token`, `application/x-www-form-urlencoded`.

Urutan:

1. Client auth (`client_secret_basic`/`post`, atau `none` + PKCE).
2. `grant_type=authorization_code`: ambil code dari Redis, validasi single-use, `client_id`, `redirect_uri`, PKCE S256, sesi masih aktif, user tidak disabled.
3. Issue access token JWT (15 menit) + ID token (5 menit).
4. Jika scope `offline_access`: buat refresh token 256-bit, simpan hash + family, absolute 30 hari.
5. Audit event `token.issued` dan respons.

Client credentials: tanpa sesi/user, hanya access token, tidak ada refresh token, `sub` = `client_id`.

### 5.4 Refresh Rotation & Reuse Detection

```
hash(refresh) ──► cari token
  ├─ tidak ada / expired / revoked ──► invalid_grant
  ├─ used_at != null ──► REUSE: revoke seluruh family + audit + invalid_grant
  └─ valid ──► satu transaksi:
        1. tandai token lama used_at + replaced_by
        2. insert token baru (hash, family_id, parent_id)
        3. hitung absolute_expires_at; tolak jika lewat
        4. audit
       ──► access + id + refresh baru
```

Reuse detection harus atomik: gunakan `SELECT ... FOR UPDATE` pada baris token atau update bersyarat `WHERE used_at IS NULL`; jika update mengenai 0 baris, perlakukan sebagai reuse/concurrent use dan cabut family.

### 5.5 Logout

- **RP-initiated:** `GET /realms/{realm}/logout?id_token_hint=...&post_logout_redirect_uri=...&state=...`. Validasi tanda tangan, `iss`, `aud`; ambil `sid`, cabut sesi, enqueue backchannel logout, redirect hanya jika URI terdaftar.
- **UI logout:** `POST /api/v1/auth/logout` (CSRF) mencabut sesi saat ini.
- **Backchannel:** worker mengirim `logout_token` JWT (`typ: logout+jwt`, klaim `iss`, `sub`, `aud`, `iat`, `jti`, `events`, `sid`) ke `backchannel_logout_uri`; retry lewat outbox.
- Revoke sesi = update `session.revoked_at` + hapus cache Redis + revoke refresh family, dalam satu method repository.

### 5.6 MFA

- Enrollment admin dipaksa: sesi ada, tapi middleware admin menolak akses sampai `mfa_totp.confirmed_at` terisi. Endpoint yang diizinkan hanya enrollment, logout, dan session.
- Enrollment: generate secret 160-bit, tampilkan QR/otpauth URI, verifikasi satu kode, lalu aktifkan + generate 10 recovery code (ditampilkan sekali, disimpan sebagai hash SHA-256).
- Verifikasi login: TOTP 30 detik, window ±1, tolak `last_used_step` yang sama, update counter.
- Recovery code single-use; setelah habis, user harus regenerate.

### 5.7 Invite, Verifikasi, Reset

- Invite: admin membuat user tanpa credential + `email_token(purpose=invite, 7 hari)`. Email berisi tautan `/accept-invite?token=...`. Menerima token = set password + `email_verified_at`, dalam satu transaksi.
- Verifikasi: `email_token(purpose=verify, 24 jam)`.
- Reset: `email_token(purpose=reset, 30 menit)`; sukses reset mencabut semua sesi dan refresh family.
- Semua respons ke user generik (anti-enumerasi); pengiriman lewat outbox agar tidak bergantung pada SMTP saat request.

### 5.8 Admin Request

- Semua `/api/v1/*` ber-cookie memakai `SessionAuth` + `CSRF` (header `X-CSRF-Token` vs cookie `sso_csrf`, diikat HMAC ke sesi).
- Role `platform-admin` dicek di middleware; perubahan role berlaku pada request berikutnya.
- Idempotency: `Idempotency-Key` disimpan di Postgres per endpoint; replay mengembalikan respons tersimpan.
- Semua aksi tulis menulis audit dalam transaksi yang sama dengan perubahan data.

## 6. Repository & Transaksi

### Aturan

- Setiap method yang menulis lebih dari satu tabel membuka transaksi sendiri lewat helper `database.WithTx` di `internal/infra/database`.
- Handler tidak menerima `pgx.Tx`; handler memanggil satu method.
- Operasi baca boleh memakai pool langsung.
- Unique constraint dan foreign key dipakai sebagai pertahanan terakhir; error dipetakan ke kode domain.
- Audit + outbox ditulis di transaksi yang sama dengan perubahan bisnis (transactional outbox).
- Idempotency disimpan di Postgres (`idempotency_key`), bukan Redis.

### Contoh method

| Method | Tabel yang disentuh dalam satu transaksi |
|---|---|
| `CreateSessionWithTokens` | `session`, `refresh_token`, `audit_event` |
| `RotateRefreshToken` | `refresh_token` (lama + baru), `audit_event` |
| `RevokeRefreshFamily` | `refresh_token` (family), `session` (opsional), `audit_event` |
| `CompletePasswordReset` | `user_credential`, `email_token`, `session`, `refresh_token`, `audit_event` |
| `AcceptInvite` | `user`, `user_credential`, `email_token`, `audit_event` |
| `DisableUserAndRevokeSessions` | `user`, `session`, `refresh_token`, `audit_event` |
| `CreateClientWithSecret` | `client`, `client_redirect_uri`, `client_secret`, `audit_event` |
| `EnqueueBackchannelLogout` | `outbox`, `audit_event` |

## 7. Redis: Key, TTL, Degradasi

| Key | Isi | TTL |
|---|---|---|
| `authreq:{id}` | auth request | 10 menit |
| `authcode:{hash}` | code + binding | 60 detik |
| `mfa:{token}` | tantangan MFA | 5 menit |
| `sess:{hash}` | cache sesi | 30 detik |
| `rl:{scope}:{key}` | counter rate limit | 1–15 menit |
| `jwks:{realm}` | cache JWKS internal (opsional) | 5 menit |

Degradasi mengikuti PRD §9: sesi fallback Postgres; auth code dan rate limit fail-closed; idempotency, consent, MFA, dan audit tetap di Postgres.

## 8. Token, Cookie, dan Kripto

### Token

| Token | Format | Lifetime | Penyimpanan |
|---|---|---|---|
| Access | JWT RS256 | 15 menit | tidak disimpan |
| ID | JWT RS256 | 5 menit | tidak disimpan |
| Refresh | opaque 256-bit | idle 30 hari, absolute 30 hari | hash SHA-256 |
| Auth code | opaque 256-bit | 60 detik | hash di Redis |
| Session ID | opaque 128-bit | sesuai matriks sesi | hash di Postgres + cache |
| Logout token | JWT RS256 | 2 menit | tidak disimpan |

Klaim access token: `iss`, `sub`, `aud` (= client_id), `exp`, `iat`, `jti`, `kid`, `scope`, `sid`, `roles`/`groups` jika scope disetujui. ID token sesuai PRD FR-OID-15.

### Cookie

| Cookie | Atribut | Fungsi |
|---|---|---|
| `__Host-sso_session` | HttpOnly, Secure, SameSite=Lax, Path=/ | sesi |
| `sso_csrf` | Secure, SameSite=Lax, Path=/, tidak HttpOnly | double-submit |
| `__Host-sso_authreq` | HttpOnly, Secure, SameSite=Lax, Path=/ | ikatan auth request |

Dev (`APP_ENV=dev`) memakai nama tanpa `__Host-` dan tanpa `Secure`; perbedaan ini hanya boleh aktif di dev dan ditolak saat produksi.

### Enkripsi

- Signing key RS256 3072-bit, disimpan terenkripsi AES-256-GCM dengan master key; format ciphertext menyimpan `key_id`.
- TOTP secret dan kredensial SMTP dienkripsi AES-256-GCM.
- Client secret dan refresh token di-hash SHA-256 (entropi 256-bit), bukan dienkripsi.
- Master keyring: `SSO_MASTER_KEYS` (JSON `{id: base64key}`) + `SSO_MASTER_KEY_ID`. Rotasi master key me-re-enkripsi ciphertext bertahap dalam transaksi per baris.

## 9. Error Model

| Konteks | Format |
|---|---|
| OIDC/OAuth endpoints | `application/json` error OAuth: `error`, `error_description`, `state` (jika relevan) |
| Admin & self-service | `application/problem+json` RFC 9457 + `code` internal |
| Halaman/redirect authorize | redirect ke `redirect_uri` dengan `error` dan `state`, bukan JSON |

Kode internal yang dipakai: `invalid_credentials`, `account_locked`, `mfa_required`, `mfa_invalid`, `session_expired`, `csrf_invalid`, `forbidden`, `not_found`, `conflict`, `rate_limited`, `idempotency_mismatch`.

Anti-enumerasi: login, forgot-password, resend invite, dan verify selalu memberi respons seragam sesuai PRD.

## 10. Checklist Keamanan

- PKCE S256 wajib; `plain` ditolak; code single-use dan terikat client/redirect.
- Redirect URI exact match; post-logout URI terdaftar; tolak open redirect.
- `state`/`nonce` divalidasi; `nonce` masuk ID token.
- CSRF double-submit + Origin check untuk endpoint ber-cookie dan pra-sesi.
- Session fixation: session ID baru setelah login dan setelah MFA.
- XSS: CSP ketat, tanpa `unsafe-inline`; Vue escape default; cookie HttpOnly.
- Clickjacking: `X-Frame-Options: DENY` untuk admin; CSP `frame-ancestors 'none'`.
- Brute force: lockout Postgres + rate limit Redis; respons seragam.
- Token leakage: `Referrer-Policy: no-referrer`; token tidak di query log; access token tidak di URL.
- SSRF: URL backchannel logout wajib HTTPS, host publik, dan bukan IP privat/loopback; validasi saat registrasi dan saat kirim.
- Header injection: validasi `redirect_uri`, `state`, nama client.
- Clock skew 60 detik; host NTP.
- Audit tidak menyimpan secret/email; log diredaksi.

## 11. Observability

- Log JSON: `time`, `level`, `msg`, `request_id`, `trace_id`, `span_id`, `user_id` (opsional), `realm`, `route`, `status`, `duration_ms`. Tidak ada email, token, atau password.
- `logger.FromContext(ctx)` menambahkan `trace_id`/`span_id` dari middleware `Tracing` dan `user_id` dari `SessionAuth`.
- Metrik: `sso_login_total{result}`, `sso_token_issued_total{grant}`, `sso_http_duration_seconds{route,status}`, `sso_outbox_pending`, `sso_sessions_active`, `sso_lockout_total`, `sso_redis_errors_total`.
- Audit ≠ log: audit adalah data bisnis di Postgres, log adalah diagnostik.
- Eksporter OTel ditunda; context W3C dan span ID tetap dibuat sejak v1.

## 12. Deployment

```
Caddy ── app (1 instance) ── Postgres
                         └─ Redis
                         └─ SMTP
```

- Migrasi dijalankan sebagai langkah terpisah (`cmd/migration up`) sebelum app start; `/readyz` gagal jika versi skema tidak cocok.
- `cmd/migration bootstrap` membuat realm default + admin pertama (password interaktif, tanpa default).
- Health: `/healthz` (proses hidup), `/readyz` (DB, Redis, versi migrasi).
- Backup `pg_dump` harian terenkripsi, retensi 7 hari; restore diuji.
- Redis tidak di-backup.
- Satu instance app di v1; jika ditambah, worker wajib advisory lock dan cache sesi TTL ≤30 detik.

### Konfigurasi utama

| Env | Contoh | Catatan |
|---|---|---|
| `APP_ENV` | `production` | `dev` mengaktifkan pengecualian cookie/localhost |
| `HTTP_ADDR` | `:8080` | listen internal |
| `EXTERNAL_URL` | `https://sso.example.com` | dasar issuer |
| `DATABASE_URL` | `postgres://...` | |
| `REDIS_URL` | `redis://...` | |
| `SSO_MASTER_KEYS` | `{"2026-01":"base64..."}` | keyring |
| `SSO_MASTER_KEY_ID` | `2026-01` | key aktif |
| `TRUSTED_PROXY_CIDRS` | `10.0.0.0/8` | |
| `LOG_LEVEL` | `info` | |
| `METRICS_ADDR` | `:9090` | |

SMTP, password policy, dan session policy disimpan per realm di Postgres dan diubah lewat admin API.

## 13. Testing

| Lapis | Cakupan | Alat |
|---|---|---|
| Unit | util, klaim token, policy password, validasi redirect | `go test` |
| Integration | repository + transaksi, seluruh alur login/token/refresh/logout, outbox | testcontainers (Postgres + Redis) |
| E2E | login, MFA, consent, reset, invite, admin CRUD | Playwright |
| Protocol | code flow, PKCE, refresh reuse, prompt/max_age, discovery | OpenID conformance suite |
| Load | login 50 RPS, `/token` 150 ms p95, Argon2 | k6 |
| Logging | field `trace_id`/`span_id`/`user_id`, propagasi `traceparent`, redaksi | integration |
| Security | redirect tampering, code reuse, CSRF, enumerasi, lockout, SSRF | integration + manual checklist |

Uji wajib khusus: Redis dimatikan (matriks degradasi), rotasi signing key, rotasi master key, last-admin protection.

## 14. ADR Ringkas

| ADR | Keputusan | Konsekuensi |
|---|---|---|
| ADR-001 | Tanpa service layer; logika di handler | Handler lebih tebal; duplikasi dikendalikan helper; test integration-first |
| ADR-002 | Transaksi di repository, coarse-grained | Method repository lebih besar; nama berbasis operasi bisnis; review ketat |
| ADR-003 | OIDC dengan `go-jose/v4`, bukan `op.Storage` | `zitadel/oidc` hanya untuk konstanta; handler dan conformance suite menanggung risiko protokol |
| ADR-004 | Access token JWT + refresh opaque | Revocation access token tertunda ≤15 menit; introspection untuk instan |
| ADR-005 | Issuer ber-prefix `/realms/{realm}` | Multi-realm tanpa migrasi issuer; URL lebih panjang |
| ADR-006 | Login/consent di SPA | Satu sistem UI; butuh CORS dan ikatan auth request |
| ADR-007 | Satu instance app di v1 | Sederhana; HA ditunda ke v1.1 |
| ADR-008 | Idempotency di Postgres | Tidak bergantung Redis; satu tabel tambahan |
| ADR-009 | Client display name ditambahkan di M4 | Tabel client mendapat `name`; API dan UI memakai nama, bukan `client_id` |

## 15. Checklist Scaffold M1

- `go.mod`, `cmd/server`, `cmd/migration`, `internal/{domain,httphandler,repository,infra,pkg}`, `web/`, `migrations/`, `deploy/`.
- Config loader + validasi env; logger slog; koneksi pgx dan Redis; health/ready.
- Migrasi awal: realm, user, credential, client, session, refresh, role, group, audit, outbox, key, idempotency.
- `bootstrap` (realm + admin) dan `seed` dev (2 client, user contoh, MailHog).
- Compose: app, Postgres, Redis, MailHog, Caddy; Makefile `dev`, `test`, `migrate`, `lint`.
- CI: vet, lint, govulncheck, test, frontend lint/typecheck/build, e2e stub.
- SPA scaffold: router publik/admin, layout, query client, halaman login kosong + health check ke API.

Kriteria selesai M1: `make dev` menyalakan seluruh stack, migrasi + bootstrap idempoten, `/healthz` dan `/readyz` hijau, CI hijau, dan scaffold SPA tampil.
