# Release Checklist v1

Status per 10 November 2026. Sumber bukti adalah suite Go, suite Playwright, k6,
dan drill lokal. Kolom status hanya hijau jika bukti sudah dijalankan.

| # | Kriteria PRD §13 | Status | Bukti |
|---|---|---|---|
| 1 | RP uji menyelesaikan login, refresh, logout, backchannel, dan CORS | Hijau | `TestIntegrationOIDCFullCodeFlow`, `TestIntegrationOIDCLogoutAndBackchannel`, `TestIntegrationOIDCDiscoveryJwksAndCORS`, `TestIntegrationOIDCConsentForThirdParty`, `web/e2e/consent.spec.ts` |
| 2 | Conformance suite profil Basic OP lulus tanpa error kritis | Blocked | Belum dijalankan. Lihat bagian Conformance. |
| 3 | Vet, lint, govulncheck, frontend gates, unit, integration, e2e hijau | Hijau | `go vet`, `golangci-lint run` 0 issue, `govulncheck` 0 vuln yang dipanggil, `npm run lint/typecheck/build`, `go test ./...` dengan Postgres/Redis, Playwright 15 lulus. CI Go job sekarang menjalankan service Postgres, Redis, dan MailHog dengan `TEST_*` env, jadi integration tidak lagi skip. |
| 4 | CSRF dan Origin wajib pada endpoint cookie dan pra-sesi | Hijau | `TestIntegrationCSRFAndOriginRejection`, `TestIntegrationAdminOriginRequired`, `TestIntegrationRedisOutageKeepsSessions` |
| 5 | Rate limit, lockout, dan degradasi Redis terbukti | Hijau | `TestIntegrationLockoutAfterFiveFailures`, `httphandler_ratelimit_test.go`, `TestIntegrationRedisOutageKeepsSessions` |
| 6 | Rotasi signing key dan master key tidak memutus token | Hijau | `TestKeyRotationKeepsOldTokensVerifying`, `TestIntegrationMasterKeyRotationKeepsSecrets` |
| 7 | Sesi lama valid saat Redis mati, login baru 503 terkontrol | Hijau | `TestIntegrationRedisOutageKeepsSessions` |
| 8 | Runbook tersedia dan sudah dicoba minimal sekali | Sebagian | `docs/RUNBOOK.md`; drill backup/restore lokal lulus. Drill staging belum. |
| 9 | Audit tanpa password/token/secret/kode MFA/email | Hijau | `TestIntegrationAdminSessionsAuditAndRevoke` memeriksa metadata dan email aktor lewat join |
| 10 | Invite, verifikasi email, reset password, forced MFA end-to-end | Hijau | `TestIntegrationAcceptInviteAndVerifyEmail`, `TestIntegrationPasswordResetRevokesSessions`, `web/e2e/password-reset.spec.ts`, `account-mfa.spec.ts`, `admin-mfa.spec.ts` |
| 11 | ID token claims, scope, `prompt=none`, `max_age` | Hijau | `TestIntegrationOIDCFullCodeFlow`, `TestIntegrationOIDCPromptNone`, `TestIntegrationOIDCMaxAge` |
| 12 | Admin terakhir tidak bisa dinonaktifkan atau kehilangan role | Hijau | `TestIntegrationLastAdminProtection` |
| 13 | Load test membuktikan batas Argon2 dan p95 login | Sebagian | Token 50 RPS p95 17,4 ms. Login 10 VU sekitar 7,1/detik p95 209 ms. 50 login/detik belum terbukti; limiter Argon2 64 MiB dengan `ARGON2_CONCURRENCY=2`. Lihat `load/README.md`. |
| 14 | Log request memuat trace/span/user dan tidak membocorkan rahasia | Hijau | `internal/infra/logger/logger_test.go`, `internal/infra/tracing/tracing_test.go`, `httphandler_middleware_test.go` |

## Drill backup/restore

Dijalankan 10 November 2026 pada Docker Postgres lokal.

- Database bersih `sso_drill` dibuat, migrasi versi 6, seed dan bootstrap berjalan.
- Backup custom-format 157.852 byte, dienkripsi AES-256-CBC dengan passphrase file.
- Restore ke `sso_restore2` dalam hitungan detik. Jumlah baris sama, versi migrasi 6.
- App dijalankan terhadap database hasil restore dengan keyring dev. `/readyz`
  `ready`, `/realms/default/jwks` mengembalikan 200 dengan satu key, jadi
  ciphertext signing key terbaca.
- Catatan: backup harus disimpan bersama master key yang cocok. Database dev
  lama berisi key dari beberapa keyring dan tidak bisa dipakai sebagai bukti
  restore yang bersih.

## Conformance

Belum dijalankan. Langkah dan prasyarat ada di `conformance/README.md`. Suite
OpenID resmi membutuhkan image Docker, client statis, dan sesi browser untuk
menyelesaikan test plan. Tidak ada kredensial atau target publik di environment
ini. Jalankan di host yang bisa diakses suite dan catat hasil Basic OP sebelum
rilis.

## Deploy

Template ada di `deploy/`. Deploy sebenarnya belum dijalankan karena belum ada
host, domain, dan kredensial. Jalankan langkah di `deploy/README.md`, lalu isi
hasil smoke test produksi di sini.

## Sign-off

- [ ] Conformance Basic OP hijau.
- [ ] Drill staging selesai.
- [ ] Deploy produksi dan smoke test selesai.
- [ ] 50 login/detik terbukti atau target direvisi bersama pemilik produk.
- [ ] Operator menandatangani checklist ini.
