# PRD — SSO Service (ITPRJ-0007)

| | |
|---|---|
| Status | Draft v1.1 (hasil review kematangan) |
| Pemilik | Tim Platform |
| Tanggal | 9 November 2026 |
| Target rilis | v1 dalam 8 minggu (solo) |
| Dokumen terkait | `docs/IMPLEMENTATION_PLAN.md`, `docs/ARCHITECTURE.md`, `api/openapi.yaml`, `docs/RUNBOOK.md` |

### Riwayat Revisi

| Versi | Tanggal | Perubahan |
|---|---|---|
| 1.0 | 9 Nov 2026 | Draft awal dari hasil discovery. |
| 1.1 | 9 Nov 2026 | Menutup 5 kontradiksi (lifetime sesi, revocation, degradasi Redis, MFA admin, CSRF), 12 blocker (ID token, scope/claims, `aud`, invite, request OIDC, otoritas admin, group, data model protokol, TOTP, Argon2 concurrency, rotasi master key, verifikasi conformance), menambah traceability milestone→FR, mapping acceptance→FR, dan owner pertanyaan terbuka. |

---

## 1. Ringkasan Eksekutif

SSO Service adalah Identity Provider (IdP) internal berbasis OpenID Connect yang menggantikan kebutuhan menyalin-manage user di tiap aplikasi. Layanan ini menyediakan login terpusat, manajemen user/client/role, MFA, sesi, audit, dan key management, dengan semangat "seperti Keycloak" namun sengaja dibatasi pada OIDC yang benar dan aman.

v1 dibangun sebagai satu binary Go yang juga menyajikan SPA Vue 3 untuk halaman login dan admin console, berjalan di Docker Compose dengan Postgres dan Redis, dan menargetkan skala internal (≤10.000 user, ≤50 login/detik).

## 2. Latar Belakang & Masalah

Saat ini setiap aplikasi internal mengelola user dan sesi sendiri. Akibatnya:

- User memiliki banyak kredensial dan pengalaman login yang tidak konsisten.
- Tidak ada satu tempat untuk mencabut akses ketika karyawan keluar.
- Tidak ada audit terpusat atas kejadian login dan perubahan hak akses.
- Tim aplikasi membangun ulang reset password, MFA, dan manajemen sesi dengan kualitas keamanan yang bervariasi.

Mengadopsi Keycloak penuh berarti biaya operasional dan kompleksitas fitur yang tidak dibutuhkan. Solusi yang dibangun sendiri memungkinkan kontrol penuh atas model data, kebijakan keamanan, dan UX, dengan scope yang dikunci.

## 3. Tujuan & Metrik Keberhasilan

### Tujuan

1. Menyediakan satu titik autentikasi OIDC yang bisa dipakai semua aplikasi internal.
2. Menjamin pencabutan akses instan melalui revoke sesi dan refresh token.
3. Menyediakan audit lengkap untuk kejadian autentikasi dan perubahan konfigurasi.
4. Menjaga standar keamanan: PKCE wajib, token berumur pendek, MFA untuk admin, rate limiting.
5. Menjadi fondasi multi-realm yang siap tumbuh tanpa migrasi issuer.

### Metrik (6 bulan pertama)

| Metrik | Target |
|---|---|
| Aplikasi internal terintegrasi | ≥3 |
| Login melalui SSO | ≥90% dari total login aplikasi terintegrasi |
| p95 respons endpoint `/token` | <150 ms |
| p95 halaman login sampai sesi dibuat | <500 ms (diukur pada beban normal, Argon2 dibatasi konkurensi) |
| Revoke sesi/refresh token (user dinonaktifkan → sesi & refresh mati) | <5 detik |
| Access token yang masih valid setelah revoke | ≤15 menit (sifat JWT; lihat catatan revocation §8.4 dan matriks degradasi §9) |
| Insiden keamanan akibat kebocoran kredensial | 0 |

## 4. Non-Goals (v1)

Secara eksplisit **tidak** dibangun di v1:

- SAML 2.0, LDAP/Active Directory federation, SCIM.
- Social login (Google, Microsoft, GitHub).
- WebAuthn/passkeys, device code flow, token exchange, CIBA.
- Resource indicators (RFC 8707) dan access token multi-audience.
- Custom theme per client, multi-bahasa lebih dari `en` + `id`.
- CRUD realm via admin UI (realm hanya dibuat lewat CLI di v1).
- Perubahan email mandiri (self-service hanya nama; ganti email lewat admin).
- Role inheritance melalui group (group hanya keanggotaan + klaim).
- gRPC dan Kafka (hanya kontrak/folder kosong).
- HSM/KMS, multi-region, active-active.
- Self-registration terbuka secara default.

## 5. Pengguna & Persona

| Persona | Kebutuhan utama |
|---|---|
| **End user** | Login cepat, lupa password, atur MFA, lihat & cabut sesi sendiri. |
| **Admin platform** | Kelola user, client, role, grup, lihat sesi aktif, audit, konfigurasi realm. |
| **Developer aplikasi** | Mendaftarkan client, mendapatkan discovery URL, memvalidasi token, menguji flow. |
| **Operator** | Deploy, backup, rotasi key, memantau metrik dan log, menangani insiden. |

## 6. Ruang Lingkup & Prioritas

### v1.0 (wajib)

- OIDC authorization code + PKCE, discovery, JWKS, userinfo, logout.
- ID token, access token JWT, refresh token rotation dengan reuse detection.
- Client credentials grant.
- Login password, verifikasi email, invite, reset password, remember me.
- TOTP MFA + recovery code; MFA wajib dan enrollment paksa untuk admin.
- Sesi server-side dengan daftar dan revoke.
- Admin API + admin console (user, client, role, grup, sesi, audit, settings realm).
- Self-service akun.
- Audit log + outbox email.
- Rate limiting, lockout, degradasi Redis yang terdefinisi.
- Signing key management + rotasi + rotasi master key.
- Bootstrap CLI, seed dev, migrasi.

### v1.1 (setelah rilis)

- CRUD realm dari admin UI.
- Consent screen yang dapat dikonfigurasi, social login, client management lanjutan.
- Kafka sebagai transport audit/event.
- Theming per client (CSS variables).
- Email template editor.
- Perubahan email mandiri dengan verifikasi ulang.

### Nanti

- SAML, LDAP/AD, SCIM, WebAuthn, device code, token exchange, CIBA, KMS/HSM.

## 7. Arsitektur Ringkas

### Stack

- **Backend:** Go, `chi` di atas `net/http`, `sqlc` + `pgx/v5`, `goose`.
- **Protokol:** `go-jose/v4` untuk JWT/JWKS dan handler OIDC sendiri; konstanta wire dari `zitadel/oidc/v3`. Fallback ini dipakai karena `op.Storage` mengasumsikan OP memiliki redirect login/consent, sedangkan UI adalah SPA. Conformance suite menjadi gate rilis.
- **Data:** PostgreSQL (sumber kebenaran), Redis (sesi cache, auth code, rate limit).
- **Antarmuka:** SPA Vue 3 + Vite + shadcn-vue, di-embed ke binary via `embed.FS`.
- **Email:** SMTP, outbox transaksional di Postgres + worker in-process.

### Prinsip

1. **Logika bisnis di handler.** `internal/domain` hanya berisi konstanta, DTO, model, dan interface.
2. **Transaksi di repository.** Operasi tulis atomik lintas entitas diekspos sebagai satu method repository bernama bisnis (mis. `CreateSessionWithTokens`, `RotateRefreshToken`, `CompletePasswordReset`). Handler tidak membuka transaksi dan tidak menyentuh `pgx` langsung.
3. **Realm sejak awal.** Semua data ber-`realm_id`, issuer ber-prefix `/realms/{realm}`.
4. **Postgres sumber kebenaran.** Sesi dan token permanen selamat jika Redis flush; alur login baru tetap membutuhkan Redis (lihat matriks degradasi §9).
5. **UI di SPA.** Halaman authorize tidak dirender Go; handler menyimpan auth request lalu redirect ke `/login?request_id=...`.

### Struktur direktori (final)

```
cmd/server/                    # HTTP server + worker
cmd/migration/                 # up, down, seed, bootstrap, rotate-key, rotate-master-key
internal/httphandler/          # REST API + OIDC routes (httphandler_*.go)
internal/grpchandler/          # kontrak kosong di v1
internal/eventhandler/         # kontrak kosong di v1
internal/workerhandler/        # outbox email, cleanup token (workerhandler_*.go)
internal/domain/               # const_*, dto_*, model_*, port_repo_*, port_ext_*
internal/repository/           # FLAT, tanpa subfolder: repo_pg_*, repo_redis_*, repo_smtp_*,
                               # repo_http_*, repo_grpc_*, repo_kafka_*, repo_oidc_storage.go
internal/infra/                # caching, logger, tracing, kafka, database, config
internal/pkg/                  # utility
migrations/  api/  web/  deploy/
```

## 8. Functional Requirements

Prioritas: **P0** wajib untuk rilis, **P1** sebaiknya ada, **P2** ditunda.

### 8.1 Realm & Multi-tenancy

| ID | Prioritas | Requirement |
|---|---|---|
| FR-RLM-01 | P0 | Sistem memiliki tabel `realm`; v1 membuat satu realm default via bootstrap. |
| FR-RLM-02 | P0 | Semua entitas bisnis menyimpan `realm_id`; tidak ada query lintas realm. |
| FR-RLM-03 | P0 | Issuer dan discovery berada di `https://{host}/realms/{realm}`. |
| FR-RLM-04 | P0 | Signing key dikelola per realm; master keyring bersifat global dan berversi (setiap ciphertext menyimpan `key_id`). |
| FR-RLM-05 | P0 | Admin dapat melihat ringkasan realm dan mengubah settings realm (SMTP, password policy, session policy). |
| FR-RLM-06 | P2 | CRUD realm dari admin UI. |

### 8.2 Identitas & Siklus Hidup User

| ID | Prioritas | Requirement |
|---|---|---|
| FR-USR-01 | P0 | Email unik per realm, disimpan case-insensitive; `username` opsional; profil menyimpan `name`, `given_name`, `family_name`, `locale`. |
| FR-USR-02 | P0 | Registrasi default invite-only; toggle self-registration per realm. |
| FR-USR-03 | P0 | Email wajib diverifikasi sebelum login pertama; token verifikasi single-use, di-hash, kedaluwarsa 24 jam. |
| FR-USR-04 | P0 | Reset password via token single-use, kedaluwarsa 30 menit; semua sesi & refresh token dicabut setelah reset; respons selalu generik (anti-enumerasi). |
| FR-USR-05 | P0 | Password minimal 12 karakter (dapat dikonfigurasi 12–64), tanpa aturan komposisi, ditolak jika ada di daftar password umum lokal atau mengandung email/nama. |
| FR-USR-06 | P0 | User dinonaktifkan (`disabled_at`) tidak bisa login dan semua sesinya dicabut seketika. |
| FR-USR-07 | P1 | Admin dapat membuat user, mengirim ulang undangan, dan memicu reset password. |
| FR-USR-08 | P2 | Anonimisasi/penghapusan user terjadwal (hard delete). |
| FR-USR-09 | P0 | Invite membuat user tanpa credential; token undangan single-use, di-hash, kedaluwarsa 7 hari; menerima token sekaligus menetapkan password dan memverifikasi email; admin dapat resend (token lama batal). |
| FR-USR-10 | P0 | Jika self-registration aktif, user dibuat `pending` dengan token verifikasi 24 jam; tidak ada sesi sebelum verifikasi. |
| FR-USR-11 | P1 | Rehash password otomatis saat parameter Argon2 berubah, dilakukan saat login sukses. |
| FR-USR-12 | P2 | Perubahan email mandiri dengan verifikasi ulang (ditunda ke v1.1). |

### 8.3 Autentikasi & Sesi

| ID | Prioritas | Requirement |
|---|---|---|
| FR-AUT-01 | P0 | Login dengan email + password; pesan error tidak membocorkan apakah email terdaftar. |
| FR-AUT-02 | P0 | Sesi server-side: cookie `__Host-` HttpOnly, Secure, SameSite=Lax berisi session ID opaque. Di dev HTTP, nama cookie tanpa prefix `__Host-` (hanya jika `APP_ENV=dev`). |
| FR-AUT-03 | P0 | Lifetime sesi mengikuti matriks: **normal** idle 30 menit/absolute 12 jam; **admin** idle 15 menit/absolute 12 jam tanpa remember-me; **remember-me** (hanya non-admin) idle 7 hari/absolute 30 hari. |
| FR-AUT-04 | P0 | Sesi paralel diizinkan; user dan admin dapat melihat dan mencabut sesi. |
| FR-AUT-05 | P0 | Rotasi session ID terjadi saat login dan saat MFA selesai (mencegah session fixation). |
| FR-AUT-06 | P0 | Logout mencabut sesi, refresh token family, dan mengarahkan kembali ke RP jika diminta. |
| FR-AUT-07 | P0 | Lockout setelah 5 percobaan gagal per akun dalam 15 menit; respons tetap seragam; unlock otomatis setelah 15 menit. |
| FR-AUT-08 | P0 | MFA TOTP + 10 recovery code single-use; recovery code di-hash SHA-256. |
| FR-AUT-09 | P0 | Admin dengan role `platform-admin` wajib MFA. Login pertama yang belum punya TOTP diarahkan ke enrollment paksa; tidak bisa mengakses `/admin` sebelum selesai. |
| FR-AUT-10 | P0 | TOTP: step 30 detik, window ±1, tolak pemakaian ulang time-step yang sama (simpan `last_used_step`), toleransi clock skew 60 detik. |
| FR-AUT-11 | P1 | Remember-me dapat dicabut per perangkat. |
| FR-AUT-12 | P2 | WebAuthn/passkeys. |
| FR-AUT-13 | P0 | Endpoint pra-sesi (`/login`, `/forgot-password`, `/reset-password`) memvalidasi `Origin`/`Referer` dan memakai CSRF token pra-sesi; login lintas-origin ditolak. |

### 8.4 OIDC / OAuth2 Provider

| ID | Prioritas | Requirement |
|---|---|---|
| FR-OID-01 | P0 | Discovery doc valid di `/realms/{realm}/.well-known/openid-configuration`; nilai `response_types_supported`, `grant_types_supported`, `scopes_supported`, `claims_supported`, `code_challenge_methods_supported` konsisten dengan implementasi dan diuji otomatis. |
| FR-OID-02 | P0 | Authorization code flow dengan PKCE S256 wajib untuk semua client; `plain` ditolak. |
| FR-OID-03 | P0 | Redirect URI dicocokkan exact (termasuk port); tanpa wildcard; `http://localhost` hanya diizinkan di dev. |
| FR-OID-04 | P0 | `state` dan `nonce` diteruskan dan divalidasi; `nonce` disimpan di auth request dan dimasukkan ke ID token. |
| FR-OID-05 | P0 | Token endpoint mendukung `authorization_code`, `refresh_token`, `client_credentials`. |
| FR-OID-06 | P0 | Access token JWT RS256 15 menit dengan `iss`, `sub`, `aud`, `exp`, `iat`, `jti`, `kid`, `scope`. |
| FR-OID-07 | P0 | Refresh token opaque 256-bit, di-hash SHA-256, rotasi wajib; reuse mencabut seluruh family. |
| FR-OID-08 | P0 | `offline_access` scope mengontrol penerbitan refresh token. |
| FR-OID-09 | P0 | JWKS mempublikasikan semua key aktif dengan `kid`. |
| FR-OID-10 | P0 | Userinfo endpoint mengembalikan klaim sesuai scope yang disetujui. |
| FR-OID-11 | P0 | RP-initiated logout (mendukung `id_token_hint`, `client_id`, `post_logout_redirect_uri` yang terdaftar) dan backchannel logout. |
| FR-OID-12 | P0 | Consent screen hanya untuk client bertanda `third_party`; consent disimpan per user+client+scope. |
| FR-OID-13 | P1 | TTL access token dan ID token per client dapat dioverride. |
| FR-OID-14 | P2 | Device code, token exchange, CIBA, SAML. |
| FR-OID-15 | P0 | ID token JWT RS256 5 menit dengan `iss`, `sub`, `aud`, `exp`, `iat`, `auth_time`, `nonce` (jika dikirim), `amr`, `acr`, `at_hash`, `sid`; klaim `email`/`email_verified` hanya jika scope email, `roles` jika scope roles. |
| FR-OID-16 | P0 | Scope yang didukung: `openid`, `profile`, `email`, `offline_access`, `roles`, `groups`. Mapping klaim: profile → `name`,`given_name`,`family_name`,`preferred_username`,`locale`; email → `email`,`email_verified`; roles → `roles`; groups → `groups`. |
| FR-OID-17 | P0 | `aud` access token v1 = `client_id` RP peminta; multi-audience ditunda. |
| FR-OID-18 | P0 | Mendukung `prompt=none` (kembalikan `login_required`/`interaction_required` sesuai kondisi), `prompt=login` (reauth paksa), `max_age`, dan `login_hint`. Profil conformance yang dipakai diverifikasi sebelum M3; jika menguji parameter lain, parameter itu naik ke P0. |
| FR-OID-19 | P0 | Authorization code TTL 60 detik, single-use, disimpan di Redis, terikat ke `client_id`, `redirect_uri`, PKCE challenge, scope, nonce, dan sesi. |
| FR-OID-20 | P0 | Client auth: `client_secret_basic` dan `client_secret_post` untuk confidential; `none` + PKCE untuk public. |
| FR-OID-21 | P0 | CORS untuk `/token`, `/userinfo`, `/revoke`, `/introspect` berdasarkan daftar origin per client; tanpa credentials; hanya method/header yang diperlukan. |
| FR-OID-22 | P0 | Introspection (RFC 7662) untuk access/refresh token dengan client auth; hasil mempertimbangkan status sesi dan user. |
| FR-OID-23 | P0 | Revocation (RFC 7009) mencabut refresh token dan menandai sesi/access token tidak aktif untuk introspection; JWT yang sudah beredar tetap valid sampai `exp`. |
| FR-OID-24 | P0 | Backchannel logout mengirim logout token JWT bertanda tangan ke `backchannel_logout_uri` client, dengan `sid` dan `sub`; kegagalan dikirim ulang lewat outbox. |
| FR-OID-25 | P0 | Hanya `response_type=code`, `response_mode=query`, dan `grant_types` yang terdaftar per client yang diterima; selain itu error OAuth yang sesuai. |

Nilai `acr`: `urn:itprj:sso:acr:pwd` (password saja) dan `urn:itprj:sso:acr:mfa` (password + TOTP). Nilai `amr`: `pwd` dan `otp`.

**Catatan revocation:** access token JWT tetap valid di RP yang memvalidasi lokal sampai 15 menit setelah revoke. Introspection selalu memeriksa status sesi/user di Postgres sehingga hasilnya langsung berubah. RP yang menuntut revocation <5 detik harus memakai introspection.

### 8.5 Manajemen Client

| ID | Prioritas | Requirement |
|---|---|---|
| FR-CLI-01 | P0 | Client public dan confidential; secret confidential di-hash dan bisa dirotasi. |
| FR-CLI-02 | P0 | Client menyimpan redirect URI (banyak baris, exact match), post-logout redirect URI, backchannel logout URI, grant types, allowed scopes, dan flag `third_party`. |
| FR-CLI-03 | P0 | Client yang dinonaktifkan langsung menolak semua grant; sesi dan token milik client tersebut dicabut. |
| FR-CLI-04 | P1 | Riwayat rotasi secret (hanya metadata, bukan nilai). |
| FR-CLI-05 | P0 | Client menyimpan daftar CORS origin yang diizinkan untuk endpoint OIDC. |
| FR-CLI-06 | P1 | Penghapusan client permanen hanya jika tidak ada sesi/consent aktif; jika ada, client harus dinonaktifkan lebih dulu. |

### 8.6 Role, Group & Otorisasi Admin

| ID | Prioritas | Requirement |
|---|---|---|
| FR-ROLE-01 | P0 | Role bersifat per realm; `platform-admin` adalah role realm dan hanya berlaku di realm tersebut. |
| FR-ROLE-02 | P0 | Sistem menolak menonaktifkan, menghapus, atau mencabut role dari admin terakhir di realm (last-admin protection). |
| FR-ROLE-03 | P0 | Role dan group dapat diberikan/dicabut oleh admin; perubahan langsung memengaruhi token berikutnya (bukan token yang sudah terbit). |
| FR-GRP-01 | P0 | Group adalah kumpulan user tanpa inheritance role; admin dapat CRUD group dan keanggotaan. |
| FR-GRP-02 | P0 | Keanggotaan group dikirim sebagai klaim `groups` hanya jika scope `groups` disetujui. |

### 8.7 Self-Service Akun

| ID | Prioritas | Requirement |
|---|---|---|
| FR-ACC-01 | P0 | User melihat profil dan mengubah nama. |
| FR-ACC-02 | P0 | User mengganti password dengan konfirmasi password lama. |
| FR-ACC-03 | P0 | User mendaftarkan/menghapus TOTP dan membuat ulang recovery code. |
| FR-ACC-04 | P0 | User melihat sesi aktif dan mencabutnya. |

### 8.8 Admin API & Console

| ID | Prioritas | Requirement |
|---|---|---|
| FR-ADM-01 | P0 | REST `/api/v1`, OpenAPI-first, auth cookie sesi + CSRF double-submit. |
| FR-ADM-02 | P0 | Hanya user dengan role `platform-admin` yang boleh mengakses; MFA wajib dan sudah ter-enroll. |
| FR-ADM-03 | P0 | Error memakai RFC 9457 `application/problem+json`. |
| FR-ADM-04 | P0 | Pagination keyset (`after`, `limit`); `Idempotency-Key` untuk operasi create. |
| FR-ADM-05 | P0 | Layar: dashboard, users, clients, roles & groups, sessions, audit, settings realm. |
| FR-ADM-06 | P1 | Import user CSV (opsional di M5; boleh digeser ke v1.1). |
| FR-ADM-07 | P1 | Viewer signing key (metadata + `kid`, tanpa private key; opsional). |
| FR-ADM-08 | P0 | Admin hanya dapat mengelola realm tempat ia memiliki role; tidak ada admin global di v1. |
| FR-ADM-09 | P0 | Aksi admin yang mengubah keamanan (disable user, rotate secret, ubah settings) tercatat di audit. |

### 8.9 Email

| ID | Prioritas | Requirement |
|---|---|---|
| FR-EML-01 | P0 | Semua email dikirim lewat outbox transaksional Postgres + worker. |
| FR-EML-02 | P0 | Retry 3x dengan backoff eksponensial; kegagalan permanen tercatat. |
| FR-EML-03 | P0 | Template verifikasi, invite, reset password, dan peringatan keamanan; embedded, i18n `en`/`id`. |
| FR-EML-04 | P0 | SMTP dikonfigurasi per realm; MailHog untuk development. |

### 8.10 Audit

| ID | Prioritas | Requirement |
|---|---|---|
| FR-AUD-01 | P0 | Audit append-only: login sukses/gagal, logout, token issued/revoked, CRUD user/client/role/group, perubahan MFA, aksi admin. |
| FR-AUD-02 | P0 | Audit tidak pernah memuat password, token, secret, kode MFA, serta alamat dan isi email; viewer menampilkan email lewat join ke user. |
| FR-AUD-03 | P0 | Retensi 90 hari dengan job cleanup; admin dapat memfilter berdasarkan waktu, aktor, tipe event. |
| FR-AUD-04 | P1 | Export CSV. |
| FR-AUD-05 | P0 | `actor_type` membedakan `user`, `admin`, `client`, `system`. |

### 8.11 Key Management

| ID | Prioritas | Requirement |
|---|---|---|
| FR-KEY-01 | P0 | Private key RS256 3072-bit disimpan terenkripsi AES-256-GCM dengan master key dari env. |
| FR-KEY-02 | P0 | Rotasi via `cmd/migration rotate-key`; key lama tetap di JWKS sampai TTL token terlama + skew (24 jam). |
| FR-KEY-03 | P1 | Interface `KeyStore` siap ditukar ke KMS. |
| FR-KEY-04 | P0 | Master key berversi (`SSO_MASTER_KEY_ID` + keyring); `rotate-master-key` me-re-enkripsi semua secret tanpa downtime; key lama dipertahankan sampai re-enkripsi selesai. |

## 9. Non-Functional Requirements

### Keamanan

- Password: Argon2id (64 MiB, t=3, p=4, salt 16 byte), dengan batas konkurensi hashing (default 4) dan antrean; kelebihan beban mengembalikan 429/503 terkontrol, bukan OOM.
- Client secret & refresh token: random 256-bit, di-hash SHA-256.
- TOTP secret dan kredensial SMTP dienkripsi AES-256-GCM.
- CSRF double-submit hanya untuk endpoint yang memakai cookie sesi (API non-OIDC). Endpoint OIDC memakai autentikasi client dan CORS allow-list, bukan CSRF.
- CSP ketat; HSTS di proxy; cookie `__Host-` di produksi.
- `X-Forwarded-*` hanya dipercaya dari CIDR proxy.
- Rate limit: login per akun+IP, token endpoint per client, endpoint email per akun. Nilai awal: login 10/menit/IP dan 5/15 menit/akun (lockout), token 60/menit/client, email 3/15 menit/akun.
- Tidak ada kredensial default; bootstrap interaktif.
- Token dan secret tidak pernah ditulis ke log; label metrik tidak memuat email/user ID.
- SMTP wajib TLS dengan verifikasi sertifikat; kredensial SMTP disimpan terenkripsi.
- Clock skew JWT dan TOTP 60 detik; host wajib NTP.

### Degradasi Redis (wajib diuji)

| Fitur | Redis mati |
|---|---|
| Baca/tulis sesi | Fallback Postgres; sesi lama tetap valid. |
| Revoke sesi | Postgres + hapus cache best-effort; TTL cache ≤30 detik. |
| Auth code | Tidak tersedia; request `/authorize` dan completion gagal 503 terkontrol. Login baru tidak bisa diselesaikan. |
| Rate limit | Fail-closed untuk endpoint autentikasi (503 terkontrol); endpoint baca tetap jalan. |
| Idempotency key | Disimpan di Postgres, tidak bergantung Redis. |
| Consent & MFA | Postgres; tidak terpengaruh. |

### Performa

- ≤10.000 user, ≤50 login/detik, p95 `/token` <150 ms, p95 login <500 ms pada beban normal.
- Load test wajib membuktikan batas Argon2 sebelum M6.
- Redis flush tidak boleh menyebabkan kehilangan sesi permanen atau error 5xx pada sesi lama.

### Ketersediaan & Operasional

- v1: **satu instance app**, single region, Docker Compose (app, Postgres, Redis, MailHog, Caddy). Postgres/Redis boleh eksternal.
- Jika instance app ditambah (v1.1), worker wajib memakai Postgres advisory lock dan invalidasi cache sesi memakai TTL ≤30 detik + pub/sub; PRD ini tidak menjanjikan HA di v1.
- Binary distroless, konfigurasi via env + YAML opsional.
- `/healthz`, `/readyz` (termasuk cek versi migrasi), `/metrics`, graceful shutdown.
- Migrasi dijalankan sebagai langkah terpisah, bukan otomatis saat boot.
- Backup `pg_dump` harian, retensi 7 hari, terenkripsi dan disimpan terpisah dari database; Redis tidak di-backup; prosedur restore diuji minimal sekali.
- Worker cleanup: sesi kedaluwarsa, refresh token, email token, audit >90 hari, outbox gagal permanen.

### Observability

- Log JSON via `log/slog` wajib memuat `request_id`, `trace_id`, `span_id`, dan `user_id` bila request sudah terautentikasi; redaksi email/token.
- Trace context memakai W3C Trace Context (`traceparent`); jika tidak ada, dibuat di middleware. Eksporter OpenTelemetry ditunda sampai ada collector.
- Metrik Prometheus: login sukses/gagal, token issuance, error, latency, outbox backlog, sesi aktif, lockout, auth code gagal.
- Dashboard ringkas: login rate, failure rate, p95 latency, outbox backlog, sesi aktif.

### Privasi

- Minimisasi data: hanya email, nama, dan metadata sesi yang disimpan.
- Retensi audit 90 hari; penghapusan user mengikuti prosedur manual (UU PDP) sampai fitur anonimisasi tersedia.

### Aksesibilitas & UX

- Halaman login dan admin dapat digunakan dengan keyboard dan screen reader dasar (target WCAG 2.1 AA untuk halaman login).
- Bahasa `en` dan `id`, default mengikuti browser.
- Dark mode disediakan oleh shadcn-vue.

## 10. Data Model (ringkas)

| Tabel | Isi penting |
|---|---|
| `realm` | nama, issuer, settings SMTP/password/session |
| `user` | realm_id, email (unik case-insensitive), username, name, given_name, family_name, locale, email_verified_at, disabled_at |
| `user_credential` | user_id, password hash, algoritma, parameter, updated_at |
| `client` | realm_id, tipe, grant types, allowed scopes, third_party, TTL override, backchannel_logout_uri |
| `client_redirect_uri` | client_id, uri, jenis (`authorize`/`post_logout`) — exact match |
| `client_cors_origin` | client_id, origin |
| `client_secret` | client_id, hash, created_at, revoked_at |
| `role`, `user_role` | role per realm dan pemetaan ke user |
| `group`, `user_group` | grup dan keanggotaan (tanpa inheritance) |
| `session` | user_id, realm_id, IP, user agent, amr, auth_time, last_seen_at, remember_me, idle/absolute expiry, revoked_at |
| `refresh_token` | session_id, hash, family_id, parent_id, expires_at, absolute_expires_at, revoked_at |
| `consent` | user_id, client_id, scope, granted_at (unik per user+client) |
| `mfa_totp` | user_id, secret terenkripsi, last_used_step, confirmed_at |
| `mfa_recovery_code` | user_id, hash SHA-256, used_at |
| `email_token` | user_id, purpose (`verify`/`reset`/`invite`), hash, expires_at, used_at, created_by |
| `audit_event` | realm_id, actor_type, actor_id, tipe, metadata JSON, created_at; index waktu+tipe |
| `outbox` | tipe, payload, status, attempts, next_attempt_at |
| `realm_signing_key` | realm_id, kid, private key terenkripsi, aktif, created_at, retire_at |
| `idempotency_key` | key, endpoint, request hash, response, created_at, expires_at |

Auth code, rate limit counter, dan session cache hanya di Redis. Semua tabel bisnis memiliki `realm_id` (langsung atau lewat parent).

## 11. Permukaan API

### OIDC (publik)

```
GET  /realms/{realm}/.well-known/openid-configuration
GET  /realms/{realm}/jwks
GET  /realms/{realm}/authorize
POST /realms/{realm}/token
GET  /realms/{realm}/userinfo
POST /realms/{realm}/revoke
POST /realms/{realm}/introspect
GET  /realms/{realm}/logout
POST /realms/{realm}/logout/backchannel
```

### Sesi & autentikasi UI

```
POST /api/v1/auth/login
GET  /api/v1/auth/csrf
POST /api/v1/auth/mfa/verify
POST /api/v1/auth/mfa/enroll
POST /api/v1/auth/consent
POST /api/v1/auth/logout
POST /api/v1/auth/forgot-password
POST /api/v1/auth/reset-password
POST /api/v1/auth/verify-email
POST /api/v1/auth/accept-invite
GET  /api/v1/auth/session
GET  /api/v1/auth/authorize/request
POST /api/v1/auth/authorize/complete
```

### Self-service

```
GET   /api/v1/me
PATCH /api/v1/me
POST  /api/v1/me/password
GET   /api/v1/me/sessions
DELETE /api/v1/me/sessions/{id}
POST  /api/v1/me/mfa/totp
DELETE /api/v1/me/mfa/totp
POST  /api/v1/me/mfa/recovery-codes
```

### Admin

```
GET/POST        /api/v1/users
GET/PATCH       /api/v1/users/{id}
POST            /api/v1/users/{id}/reset-password
POST            /api/v1/users/{id}/invite
POST            /api/v1/users/{id}/roles
GET/POST        /api/v1/clients
GET/PATCH/DELETE /api/v1/clients/{id}
POST            /api/v1/clients/{id}/rotate-secret
GET/POST        /api/v1/roles
GET/POST        /api/v1/groups
GET/PATCH/DELETE /api/v1/groups/{id}
GET/POST        /api/v1/groups/{id}/members
DELETE          /api/v1/groups/{id}/members/{user_id}
GET             /api/v1/sessions
DELETE          /api/v1/sessions/{id}
GET             /api/v1/audit
GET/PATCH       /api/v1/realm/settings
```

## 12. UX & Halaman

### Publik
- `/login` (email/password + langkah MFA), `/login?request_id=...` untuk melanjutkan authorize.
- `/login/mfa-enroll` (enrollment paksa admin).
- `/consent` untuk client third-party.
- `/forgot-password`, `/reset-password?token=...`, `/verify-email?token=...`, `/accept-invite?token=...`.
- `/mfa/setup`.

### Akun
- `/account/profile`, `/account/security` (password, MFA, recovery code), `/account/sessions`.

### Admin
- `/admin` dashboard, `/admin/users`, `/admin/clients`, `/admin/roles`, `/admin/groups`, `/admin/sessions`, `/admin/audit`, `/admin/settings`.
- Import CSV dan key viewer ditandai opsional (P1).

Struktur frontend: `web/src/{routes,features/{auth,admin,account},components/ui,lib,styles}`; shadcn-vue di `components/ui`; TanStack Query di `lib/query`; tipe API hasil generate OpenAPI di `lib/api`.

## 13. Kriteria Penerimaan Rilis v1

| # | Kriteria | FR terkait |
|---|---|---|
| 1 | RP uji (satu SPA publik + satu backend confidential) menyelesaikan login, refresh, logout, dan backchannel logout; CORS SPA RP berfungsi. | FR-OID-02..11, FR-OID-21, FR-OID-24 |
| 2 | Conformance suite (profil Basic OP) lulus tanpa error kritis; profil dan daftar test diverifikasi sebelum M3. | FR-OID-01..25 |
| 3 | `go vet`, `golangci-lint`, `govulncheck`, lint/typecheck/build frontend, unit, integration (testcontainers), dan e2e Playwright hijau. | NFR |
| 4 | Semua endpoint ber-cookie sesi dan endpoint pra-sesi (`/login`, `/forgot-password`, `/reset-password`) menolak request tanpa CSRF/Origin valid; endpoint OIDC diuji terpisah memakai error format OAuth. | FR-ADM-01, FR-AUT-13, FR-OID |
| 5 | Rate limit, lockout, dan degradasi Redis sesuai matriks §9 terbukti lewat test otomatis. | FR-AUT-07, NFR |
| 6 | Rotasi signing key dan rotasi master key terbukti tidak memutus RP/token yang sudah ada. | FR-KEY-01..04 |
| 7 | Sesi lama tetap valid saat Redis dimatikan; login baru gagal 503 terkontrol; tidak ada 5xx tak terduga. | NFR degradasi |
| 8 | Runbook (backup, restore, rotasi key, revoke sesi, deploy) tersedia dan sudah dicoba minimal sekali. | NFR |
| 9 | Audit tidak memuat password/token/secret/kode MFA/alamat & isi email pada pemeriksaan sampel. | FR-AUD-02 |
| 10 | Alur invite, verifikasi email, reset password, dan enrollment paksa MFA admin berjalan end-to-end. | FR-USR-03/04/09, FR-AUT-09 |
| 11 | ID token memuat klaim wajib; scope `profile`/`email`/`roles`/`groups` menghasilkan klaim yang benar; `prompt=none` dan `max_age` berperilaku sesuai spec. | FR-OID-15..18 |
| 12 | Admin terakhir tidak bisa dinonaktifkan/dihapus/dicabut role-nya. | FR-ROLE-02 |
| 13 | Load test membuktikan batas konkurensi Argon2 dan p95 login. | NFR performa |
| 14 | Setiap log request memuat `trace_id`, `span_id`, dan `user_id` bila terautentikasi; tidak memuat email/token/password. | NFR observability |

## 14. Milestone & Timeline

| Milestone | Minggu | Isi | Exit criteria | FR utama |
|---|---|---|---|---|
| M1 Fondasi | 1–2 | Skeleton, config, skema + migrasi, CI, Compose, bootstrap CLI | `make dev` menjalankan app + DB + Redis + MailHog; migrasi & bootstrap jalan | FR-RLM-01..04, NFR ops |
| M2 Autentikasi inti | 3 | Login, sesi, password policy, verifikasi email, invite, reset, outbox | User bisa login, verifikasi, menerima invite, reset; email terkirim via MailHog | FR-USR-01..10, FR-AUT-01..07/13, FR-EML-01..04 |
| M3 OIDC | 4–5 | Authorize, token, ID token, JWKS, userinfo, logout, discovery, key + rotasi, refresh rotation, CORS, prompt/max_age | RP uji menyelesaikan code flow, refresh, logout; discovery valid | FR-OID-01..12, FR-OID-15..25, FR-KEY-01..03, FR-CLI-05 |
| M4 Admin & MFA | 6 | Admin API + console, self-service, TOTP + recovery, forced enrollment, last-admin, settings realm | Admin mengelola user/client/role/group; user mengaktifkan MFA; admin terakhir terlindungi | FR-AUT-08..10, FR-ACC-01..04, FR-ADM-01..05/08/09, FR-RLM-05, FR-CLI-01..05, FR-ROLE-01..03, FR-GRP-01..02 |
| M5 Hardening | 7 | Audit + viewer, rate limit/lockout, client credentials, CSP/CSRF/headers, cleanup worker | Semua gate keamanan dan test hijau; degradasi Redis teruji | FR-AUD-01..05, FR-OID-20..24, FR-CLI-06, FR-KEY-04, NFR |
| M6 Rilis | 8 | Conformance, e2e, load test, runbook, deploy produksi | Kriteria penerimaan §13 terpenuhi | Semua |

P1 boleh digeser ke v1.1 tanpa memengaruhi kriteria penerimaan: FR-ADM-06 (import CSV), FR-ADM-07 (key viewer), FR-AUD-04 (export audit), FR-USR-11, FR-AUT-11, FR-OID-13, FR-CLI-04, FR-KEY-03; semuanya dikerjakan di M5 bila waktu memungkinkan. P2 (FR-RLM-06, FR-USR-08/12, FR-AUT-12, FR-OID-14) tidak dipetakan ke milestone. Risiko jadwal terbesar di M3; yang boleh digeser adalah P1, bukan kualitas protokol.

Rincian task, estimasi jam, dependency, dan buffer ada di `docs/IMPLEMENTATION_PLAN.md`. Ringkasnya: 228 jam kerja dengan asumsi 30 jam/minggu; pada 25 jam/minggu durasi realistis 9–10 minggu.

## 15. Risiko & Mitigasi

| Risiko | Dampak | Mitigasi |
|---|---|---|
| Bug protokol OIDC | RP tidak bisa login, celah keamanan | `go-jose` + handler sendiri, conformance suite, e2e dengan RP nyata |
| Profil conformance mengharuskan fitur yang belum ada | Rilis M6 gagal | Verifikasi daftar test sebelum M3; naikkan `prompt=none`/`max_age` ke P0 (sudah dilakukan) |
| Logika tersebar di handler | Duplikasi saat gRPC/Kafka masuk | Repository coarse-grained; ekstrak helper saat transport baru ditambahkan |
| Transaksi di repository | Method repository membengkak | Nama method berbasis operasi bisnis, review wajib untuk perubahan skema tulis |
| Redis sebagai single point | Login gagal massal | Matriks degradasi §9; fallback Postgres; health check dan alert |
| Argon2 64 MiB × beban puncak | OOM / latency | Batas konkurensi + antrean + load test sebelum M6 |
| Rotasi key salah | Semua token invalid | Key lama tetap di JWKS 24 jam; uji otomatis; runbook |
| Rotasi master key salah | Secret tidak terbaca | Keyring berversi + re-enkripsi bertahap + backup |
| Scope melebar ke fitur Keycloak lain | Rilis mundur | Non-goals §4 mengikat; perubahan scope lewat revisi PRD |
| Solo developer | Bus factor | Dokumentasi arsitektur, runbook, ADR |

## 16. Asumsi & Pertanyaan Terbuka

**Asumsi:** TLS diakhiri Caddy; single region; satu instance app di v1; hanya satu realm di v1; lisensi privat tanpa file LICENSE; host memakai NTP.

| Pertanyaan | Owner | Tenggat |
|---|---|---|
| Nama domain final | Pemilik produk | Sebelum deploy M6 (dibutuhkan lebih awal untuk uji RP) |
| Penyedia SMTP produksi | Pemilik produk | Sebelum M2 |
| RP pertama yang diintegrasikan | Pemilik produk | Sebelum M3 |
| Daftar test profil conformance Basic OP | Implementer | Sebelum M3 |
| Sumber daftar password umum lokal (mis. SecLists top 100k) | Implementer | Sebelum M2 |
| Apakah MFA diwajibkan untuk semua user | Pemilik produk | Keputusan v1.1 |

## 17. Glossary

| Istilah | Arti |
|---|---|
| Realm | Isolasi logis berisi user, client, role, dan key sendiri. |
| RP | Relying Party, aplikasi yang memakai SSO. |
| Access token | JWT berumur pendek untuk mengakses API RP. |
| ID token | JWT yang membuktikan hasil autentikasi ke RP. |
| Refresh token | Token opaque untuk memperbarui access token, selalu dirotasi. |
| Auth code | Kode sekali pakai (60 detik) yang ditukar di token endpoint. |
| Auth request | State permintaan authorize yang disimpan sebelum user login. |
| Consent | Persetujuan user atas scope untuk client third-party. |
| Outbox | Tabel antrean transaksional untuk pengiriman email/event. |
| Coarse-grained repository | Method repository yang menjalankan satu operasi bisnis utuh dalam satu transaksi. |
| Platform admin | User dengan role `platform-admin` di suatu realm. |
| Remember-me | Opsi memperpanjang sesi non-admin (idle 7 hari, absolute 30 hari). |
