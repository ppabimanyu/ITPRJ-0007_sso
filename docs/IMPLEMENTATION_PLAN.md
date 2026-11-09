# IMPLEMENTATION PLAN — SSO Service (ITPRJ-0007)

| | |
|---|---|
| Status | Draft v1.1 (revisi UI platform) |
| Tanggal | 9 November 2026 |
| Acuan | `docs/PRD.md` v1.1, `docs/ARCHITECTURE.md` v1.0, `api/openapi.yaml` |
| Target | 8,6 minggu, solo |
| Kapasitas asumsi | 30 jam/minggu (259 jam) |
| Total estimasi kerja | 247 jam |
| Buffer | 12 jam (±5%) |

> **Catatan kapasitas:** angka 8 minggu hanya berlaku pada ~30 jam/minggu. Jika kapasitas riil 25 jam/minggu, durasi realistis menjadi 9–10 minggu dengan urutan tugas yang sama. Buffer 12 jam sengaja tipis; kegagalan conformance yang besar akan menggeser rilis satu minggu.

> **Revisi v1.1 (9 November 2026):** M1–M4 sudah selesai dan merge ke `main` (PR #1–#4, 180 jam termasuk Fase 0). Revisi ini menambah **M4.5 — UI Platform (19 jam)** sebelum M5: Playwright smoke lebih dulu, lalu upgrade Tailwind 4 dan regenerasi komponen dengan `shadcn-vue`. Sisa pekerjaan menjadi 67 jam terhadap 60 jam kapasitas yang tersisa, sehingga rilis bergeser ±7 jam (≈1 hari kerja). Total rencana menjadi 247 jam.

### Riwayat revisi

| Versi | Tanggal | Perubahan |
|---|---|---|
| v1.0 | 9 November 2026 | Rencana awal 8 minggu (228 jam). |
| v1.1 | 9 November 2026 | M1–M4 selesai. Tambah M4.5 UI Platform: Playwright smoke, Tailwind 4, shadcn-vue. Total 247 jam. |

---

## 1. Strategi Eksekusi

- **Vertical slice per milestone.** Setiap milestone berakhir dengan sesuatu yang bisa didemokan, bukan kumpulan modul setengah jadi.
- **Backend dulu, UI menyusul per milestone.** UI tidak menunggu seluruh API selesai; halaman login/consent masuk saat endpoint auth stabil.
- **UI kanonik.** Semua primitive UI memakai `shadcn-vue` di atas Tailwind 4; `web/src/components/ui` hanya berisi output CLI. Wrapper aplikasi (mis. `AdminPager`) tetap tipis dan dibangun dari komponen kanonik.
- **Test sebelum refactor UI.** Playwright smoke (M4.5-01) wajib hijau sebelum migrasi Tailwind 4/shadcn-vue; refactor tidak boleh mengubah perilaku.
- **P0 only.** Semua P1 (import CSV, key viewer, export audit, dll.) tidak dijadwalkan. P1 hanya dikerjakan bila buffer tersisa.
- **Trunk-based.** Branch pendek per task, PR kecil, CI wajib hijau.
- **Keamanan diuji negatif.** Setiap fitur auth wajib punya test untuk kasus gagal (PKCE salah, code reuse, CSRF, lockout, reuse refresh token).
- **Tidak ada big-bang integration.** RP uji (SPA + backend confidential) disiapkan sejak M3 dan dipakai sampai M6.

### Definition of Done (berlaku semua task)

1. Kode + test (unit/integration sesuai jenis task) ada dan lulus.
2. `go vet`, `golangci-lint`, `govulncheck`, build frontend (typecheck/lint/build), dan Playwright smoke (mulai M4.5) lulus.
3. Tidak ada secret/token/email di log atau error message.
4. Dokumentasi yang terdampak (`ARCHITECTURE`, `openapi.yaml`, runbook) diperbarui.
5. PR direview (self-review checklist) dan di-merge ke trunk.
6. Perilaku dapat didemokan atau diuji lewat perintah yang tercatat.

---

## 2. Timeline

Tanggal indikatif bila mulai Senin, 9 November 2026.

| Minggu | Tanggal | Milestone | Fokus | Estimasi | Demo/Exit |
|---|---|---|---|---|---|
| 1 | 9–15 Nov | M1 | Skeleton, config, logger/tracing, DB/Redis, migrasi, router, health | 30 jam | `make dev` menyalakan stack; `/healthz` hijau |
| 2 | 16–22 Nov | M1 → M2 | Bootstrap/seed, Compose, CI, SPA scaffold; mulai repo user/session | 30 jam | Admin seed bisa dibuat; halaman login tampil |
| 3 | 23–29 Nov | M2 | Password, login, sesi, CSRF, rate limit, outbox email, invite/reset | 30 jam | Login + invite + reset via MailHog |
| 4 | 30 Nov–6 Des | M2 → M3 | Selesaikan auth; client repo, key/JWKS, discovery, auth request | 30 jam | Discovery valid; authorize redirect ke SPA |
| 5 | 7–13 Des | M3 | Token, refresh rotation, ID token, userinfo, logout, CORS, RP e2e | 30 jam | RP uji menyelesaikan code flow + refresh |
| 6 | 14–20 Des | M4 | MFA TOTP, forced enrollment, admin API, last-admin | 30 jam | Admin CRUD + MFA wajib |
| 7 | 21–27 Des | M4 → M5 | Admin UI, self-service, audit viewer, hardening keamanan | 30 jam | Console lengkap; checklist keamanan hijau |
| 8 | 28 Des–3 Jan | M5 → M6 | Load test, e2e, conformance, runbook, deploy | 30 jam | Release candidate memenuhi §13 PRD |

**Risiko kalender:** minggu 7–8 jatuh pada libur akhir tahun. Jika kapasitas turun, pakai buffer atau geser rilis satu minggu; jangan memotong conformance.

### 2.1 Status per 9 November 2026

Timeline di atas adalah baseline. Per 9 November 2026, M1–M4 sudah merge; sisa pekerjaan mengikuti urutan baru berikut.

| Fase | Jam | Status | Catatan |
|---|---:|---|---|
| Fase 0 | 2 | ✅ | — |
| M1 Fondasi | 48 | ✅ | PR #1 |
| M2 Auth core | 38 | ✅ | PR #2 |
| M3 OIDC | 52 | ✅ | PR #3 |
| M4 Admin & MFA | 40 | ✅ | PR #4 |
| **M4.5 UI Platform** | **19** | ⬜ | Playwright smoke → Tailwind 4 → shadcn-vue |
| M5 Hardening | 28 | ⬜ | M5-07 memperluas suite Playwright |
| M6 Rilis | 20 | ⬜ | Conformance dan regresi dijalankan ulang setelah refactor UI |

Sisa pekerjaan: 67 jam (M4.5 19 + M5 28 + M6 20). Kapasitas tersisa 60 jam (240 − 180), sehingga rilis bergeser ±7 jam ≈ 1 hari kerja. Buffer 12 jam tetap dialokasikan untuk temuan conformance.

### Alokasi jam per milestone

| Milestone | Jam | Window |
|---|---:|---|
| Fase 0 | 2 | Minggu 1 |
| M1 Fondasi | 48 | Minggu 1–2 |
| M2 Auth core | 38 | Minggu 2–4 |
| M3 OIDC | 52 | Minggu 4–5 |
| M4 Admin & MFA | 40 | Minggu 6–7 |
| M4.5 UI Platform | 19 | Minggu 7 |
| M5 Hardening | 28 | Minggu 7–8 |
| M6 Rilis | 20 | Minggu 8–9 |
| **Total** | **247** | |

---

## 3. Breakdown Tugas

### Fase 0 — Persiapan (2 jam)

| ID | Tugas | Jam | Depends | Output/DoD |
|---|---|---:|---|---|
| P0-01 | Konfirmasi module path (default `github.com/ppa/itprj-0007-sso`), branch protection, label milestone | 2 | — | Repo siap; milestone GitHub/GitLab dibuat |

### M1 — Fondasi (48 jam)

| ID | Tugas | Jam | Depends | Output/DoD |
|---|---|---:|---|---|
| M1-01 | `go.mod`, Makefile (`dev`, `test`, `lint`, `migrate`), `.gitignore`, `.editorconfig` | 2 | P0-01 | `go test ./...` jalan |
| M1-02 | Config loader + validasi env + `.env.example` | 2 | M1-01 | Gagal cepat jika env wajib kosong; unit test |
| M1-03 | Logger slog JSON + `trace_id`/`span_id`/`user_id` + redaksi | 4 | M1-02 | Log field lengkap; unit test redaksi |
| M1-04 | Tracing W3C: parse/generate `traceparent`, middleware | 3 | M1-02 | Trace ID stabil; test propagasi |
| M1-05 | Postgres pool + goose runner + cek versi di `/readyz` | 4 | M1-02 | Migrasi jalan; readiness gagal jika versi beda |
| M1-06 | Redis client + health + helper key | 2 | M1-02 | Ping; key builder teruji |
| M1-07 | Migrasi batch 1: realm, user, credential, role, group, session, refresh | 5 | M1-05 | `migrate up/down` bersih |
| M1-08 | Migrasi batch 2: client, redirect, CORS, secret, consent, MFA, email token, key, audit, outbox, idempotency | 4 | M1-05 | Index unik + FK benar; constraint diuji |
| M1-09 | Router chi + middleware chain + health/ready + graceful shutdown | 4 | M1-03/04 | Server start/stop bersih; request ID + trace di log |
| M1-10 | Problem+json renderer + OAuth error helper + kode error internal | 2 | M1-09 | Test format RFC 9457 dan error OAuth |
| M1-11 | `bootstrap` (realm, admin, signing key) + `seed` dev (2 client, user) | 4 | M1-07/08 | Idempoten; tanpa password default |
| M1-12 | Dockerfile distroless + Compose (app, PG, Redis, MailHog, Caddy) | 3 | M1-09 | `make dev` menyalakan semua service |
| M1-13 | SPA scaffold: Vite, Vue, router, shadcn-vue, TanStack Query, i18n, embed | 4 | M1-09 | Halaman shell + health check ke API |
| M1-14 | CI: vet, golangci-lint, govulncheck, go test, frontend lint/typecheck/build | 3 | M1-01/13 | Pipeline hijau di PR pertama |
| M1-15 | Verifikasi M1 + smoke test + dokumentasi menjalankan dev | 2 | M1-11..14 | Exit criteria M1 terbukti |

**Exit M1:** `make dev` menyalakan stack, migrasi + bootstrap idempoten, `/healthz` dan `/readyz` hijau, log memuat `trace_id`/`span_id`, CI hijau, SPA shell tampil.

### M2 — Auth Core (38 jam)

| ID | Tugas | Jam | Depends | Output/DoD |
|---|---|---:|---|---|
| M2-01 | Repo user + credential; Argon2id, password policy, daftar password umum, rehash | 5 | M1-07/08 | Unit test policy + hash; email case-insensitive |
| M2-02 | Repo sesi + cookie `__Host-` + rotasi + CSRF + middleware session | 5 | M2-01 | Test fixation, idle/absolute, remember-me |
| M2-03 | Rate limit Redis + lockout Postgres + endpoint login + anti-enumerasi | 5 | M2-01/02 | Test 5 gagal/15 menit, respons seragam, 429/423 |
| M2-04 | Outbox repo + worker email + SMTP + template en/id | 5 | M1-08 | Email terkirim via MailHog; retry 3x |
| M2-05 | Invite, verifikasi email, reset password + repo email token | 5 | M2-04 | Token single-use, 7/24 jam/30 menit; cabut sesi saat reset |
| M2-06 | Self-service API: profil, ganti password, daftar/cabut sesi | 3 | M2-02 | OpenAPI sesuai; CSRF diuji |
| M2-07 | Frontend: login, forgot/reset, accept invite, verify, shell akun | 6 | M2-03..06 | Alur end-to-end via browser |
| M2-08 | Integration test auth core (testcontainers) | 4 | M2-01..05 | Login, invite, reset, lockout, outbox lulus |

**Exit M2:** user bisa login, menerima invite, verifikasi email, reset password, dan mengelola sesi; tidak ada email/token di log.

### M3 — OIDC Provider (52 jam)

| ID | Tugas | Jam | Depends | Output/DoD |
|---|---|---:|---|---|
| M3-01 | Repo client + validasi redirect exact, grant, scope, CORS origin | 4 | M1-08 | Unit test open redirect + wildcard ditolak |
| M3-02 | Repo signing key + JWT signer + JWKS + `rotate-key` | 4 | M1-08 | Dua key aktif; `kid` benar; rotasi teruji |
| M3-03 | Adapter `zitadel/oidc` (`op.Storage`) + discovery | 6 | M3-01/02 | Discovery valid; storage test |
| M3-04 | Auth request di Redis + cookie `sso_authreq` + pengikatan browser | 4 | M3-03 | Request ID tidak bisa dipakai tanpa cookie |
| M3-05 | Handler authorize + error redirect + `prompt`/`max_age`/`login_hint` | 4 | M3-04 | Redirect valid; error OAuth sesuai spec |
| M3-06 | Token endpoint: code exchange, PKCE S256, client auth basic/post | 5 | M3-05 | Code single-use; test PKCE salah |
| M3-07 | Refresh rotation + reuse detection + client credentials | 5 | M3-06 | Family dicabut saat reuse; test konkurensi |
| M3-08 | ID token + claims/scope + userinfo | 4 | M3-06 | Test klaim `amr`/`acr`/`nonce`/`at_hash`/`sid` |
| M3-09 | Logout RP-initiated + backchannel via outbox | 4 | M3-08 | Sesi dicabut; logout token terkirim |
| M3-10 | CORS endpoint OIDC + CSP/CORS test | 3 | M3-06 | Origin salah ditolak |
| M3-11 | RP uji (SPA + backend) + integration test penuh | 5 | M3-06..09 | Code flow, refresh, logout hijau |
| M3-12 | Frontend login/consent terhubung authorize + request API | 4 | M3-05, M2-07 | Login dari RP kembali ke RP |

**Exit M3:** RP uji menyelesaikan login, refresh, userinfo, logout; discovery dan JWKS valid; tidak ada token bocor di URL/log.

### M4 — Admin & MFA (40 jam)

| ID | Tugas | Jam | Depends | Output/DoD |
|---|---|---:|---|---|
| M4-01 | TOTP enrollment/verifikasi/recovery code + anti-replay | 5 | M2-02 | Test window ±1, step reuse ditolak |
| M4-02 | Forced enrollment admin + middleware akses admin | 5 | M4-01, M2-02 | Admin tanpa TOTP tidak bisa masuk `/admin` |
| M4-03 | Admin API users + last-admin protection + idempotency | 6 | M2-01 | Test admin terakhir tidak bisa dinonaktifkan |
| M4-04 | Admin API clients, roles, groups + membership | 5 | M3-01 | CRUD + validasi; audit tercatat |
| M4-05 | Admin API sessions, audit, settings realm | 4 | M2-02, M1-08 | Filter + pagination; SMTP write-only |
| M4-06 | Admin UI shell + users + forced MFA | 6 | M4-02/03 | CRUD user via browser |
| M4-07 | Admin UI clients, roles, groups, sessions | 5 | M4-04/05 | CRUD client + rotate secret tampil sekali |
| M4-08 | Self-service UI: profil, password, MFA, sesi | 4 | M2-06, M4-01 | Semua alur account via browser |

**Exit M4:** admin bisa mengelola user/client/role/group/session/settings; admin wajib MFA; last-admin terlindungi; self-service lengkap.

### M4.5 — UI Platform (19 jam)

Tujuan: UI memakai **shadcn-vue kanonik di atas Tailwind 4**, dengan Playwright smoke sebagai jaring pengaman sebelum refactor. Tidak ada perubahan perilaku produk di milestone ini.

| ID | Tugas | Jam | Depends | Output/DoD |
|---|---|---:|---|---|
| M4.5-01 | Playwright: config, fixture login (password + MFA), job CI; smoke login, MFA, consent, admin user CRUD, client create | 4 | M4, stack Compose | Suite hijau di lokal + CI; screenshot saat gagal |
| M4.5-02 | Upgrade Tailwind 3 → 4: plugin `@tailwindcss/vite`, `@import "tailwindcss"`, token ke `@theme inline`, hapus `tailwind.config.js`/`postcss.config.js`, `tw-animate-css` | 3 | M4.5-01 | Build lulus; seluruh halaman tampil sama; tidak ada utility class hilang |
| M4.5-03 | `shadcn-vue@2.9` pada `components.json` (style v2, `data-slot`, `lucide-vue-next`), regenerasi 11 primitive | 2 | M4.5-02 | `components/ui` 100% output CLI; `iconLibrary` konsisten |
| M4.5-04 | Pertahankan delta aplikasi: variant `success` pada Alert, `Input.focus()` untuk kode OTP/MFA, token warna dan locale | 1 | M4.5-03 | Semua pemakaian lama typecheck; tidak ada regresi visual |
| M4.5-05 | Migrasi `DialogPanel` → `Dialog*`; `AdminConfirmDialog` → `AlertDialog`; 6 call site | 2 | M4.5-03 | `DialogPanel.vue` dihapus; dialog komposabel; focus trap dan Esc teruji |
| M4.5-06 | Migrasi Select native → `Select*` (5 form admin + language switch + profil); sentinel untuk opsi "semua" | 2 | M4.5-03 | Tidak ada `<select>` mentah; keyboard nav dan placeholder benar |
| M4.5-07 | Adopsi komponen kanonik pada layar nyata: `DropdownMenu` (aksi baris tabel), `Tabs`, `Tooltip`, `Separator`, `Skeleton`, `Checkbox` | 3 | M4.5-05/06 | Setiap komponen dipakai minimal di satu layar; tidak ada komponen mati |
| M4.5-08 | Verifikasi: typecheck, lint, build, `go build`, Playwright smoke, browser pass semua halaman (desktop + mobile 380px) | 2 | M4.5-01..07 | Gate M4.5 hijau; screenshot before/after dilampirkan di PR |

**Catatan implementasi M4.5**

- CLI: `shadcn-vue@2.9.0` (jalur Tailwind 4). Komponen yang dipasang: `alert`, `alert-dialog`, `badge`, `button`, `card`, `checkbox`, `dialog`, `dropdown-menu`, `input`, `label`, `select`, `separator`, `skeleton`, `switch`, `table`, `tabs`, `textarea`, `tooltip`. Komponen yang belum dipakai tidak dipasang.
- Tailwind 4: `src/styles/index.css` memakai `@import "tailwindcss"`, `@theme inline`, dan `tw-animate-css`. Token warna lama dipertahankan (dipetakan ke `@theme inline`) agar tampilan tidak berubah.
- `components.json` disesuaikan ke format v2 (`tailwind.config` kosong, `iconLibrary: lucide`); `lucide-vue-next` ditambahkan dan dipakai komponen kanonik.
- Dialog: `DialogPanel` dihapus; konfirmasi destruktif memakai `AlertDialog`; dialog form memakai `Dialog` + `DialogHeader`/`DialogFooter`.
- Select: `SelectItem` tidak boleh bernilai `""`; opsi "semua" memakai sentinel `all` lalu di-map ke string kosong. `Input` tetap mengekspos `focus()` untuk auto-focus kode OTP/MFA.
- Pengecualian yang dipertahankan: `AdminPager` tetap wrapper aplikasi karena paginasi API berbasis cursor, bukan nomor halaman. `AdminConfirmDialog`, `AdminSecretDialog`, dan `AdminProblem` tetap wrapper tipis di atas komponen kanonik.

**Exit M4.5:** Tailwind 4 aktif, `components/ui` seluruhnya hasil `shadcn-vue`, tidak ada primitive custom, Playwright smoke hijau, dan seluruh alur M2–M4 tetap berjalan.

### M5 — Hardening (28 jam)

| ID | Tugas | Jam | Depends | Output/DoD |
|---|---|---:|---|---|
| M5-01 | Audit event coverage + viewer + cleanup 90 hari | 4 | M4-05 | Semua event PRD tercatat; tanpa secret/email |
| M5-02 | Rate limit seluruh endpoint + verifikasi client credentials | 4 | M3-07 | Test per client/IP; 429 konsisten |
| M5-03 | Security headers, CSP, CSRF, Origin check, cookie dev/prod | 4 | M2-02 | Checklist §10 ARCHITECTURE hijau |
| M5-04 | Degradasi Redis: fallback sesi, fail-closed auth, test | 3 | M2-02 | Sesi lama valid; login baru 503 terkontrol |
| M5-05 | Rotasi signing key + master key + test tanpa downtime | 4 | M3-02 | Token lama tetap valid; secret terbaca |
| M5-06 | Load test k6 (login, token) + tuning Argon2 | 4 | M2, M3 | p95 sesuai target atau bottleneck terdokumentasi |
| M5-07 | Perluas Playwright dari M4.5-01: reset password, enrollment MFA, admin roles/groups/sessions/settings, kasus negatif | 3 | M4.5-01 | Semua alur kritis hijau |
| M5-08 | Cleanup worker: sesi/token/outbox gagal | 2 | M2-04 | Job idempoten; metrik backlog |

**Exit M5:** seluruh gate keamanan hijau, degradasi Redis terbukti, load test terdokumentasi, audit lengkap.

### M6 — Rilis (20 jam)

| ID | Tugas | Jam | Depends | Output/DoD |
|---|---|---:|---|---|
| M6-01 | Setup OpenID conformance suite + perbaikan temuan | 6 | M3 | Profil Basic OP lulus tanpa error kritis |
| M6-02 | Regresi e2e + integration penuh + perbaikan | 4 | M5 | Semua test hijau |
| M6-03 | Runbook: deploy, backup/restore, rotasi key, revoke sesi | 3 | M5-05 | Sudah dijalankan minimal sekali di staging |
| M6-04 | Deploy produksi (Caddy + app + PG + Redis) | 3 | M6-03 | Smoke test produksi |
| M6-05 | Backup/restore drill | 2 | M6-04 | Data kembali; waktu tercatat |
| M6-06 | Release checklist + sign-off kriteria §13 PRD | 2 | Semua | Checklist terisi |

**Exit M6:** kriteria penerimaan PRD §13 terpenuhi; release candidate siap dipakai RP pertama.

---

## 4. Dependency & Critical Path

```
P0-01
  └─ M1-01 → M1-02 → M1-03/04/05/06 → M1-07/08 → M1-11
        └─ M1-09 → M1-10/12/13/14 → M1-15
              └─ M2-01 → M2-02 → M2-03 → M2-07
                    └─ M2-04 → M2-05 → M2-08
                          └─ M3-01/02 → M3-03 → M3-04 → M3-05 → M3-06
                                └─ M3-07/08 → M3-09/10/11 → M3-12
                                      └─ M4-01 → M4-02/03/04/05 → M4-06/07/08
                                            └─ M4.5-01 → M4.5-02 → M4.5-03 → M4.5-04/05/06 → M4.5-07 → M4.5-08
                                                  └─ M5-01..08
                                                        └─ M6-01..06
```

Critical path: **skema + sesi + authorize/token + MFA admin + UI platform + conformance**. Frontend bukan critical path fitur, tetapi M4.5 menjadi gate sebelum M5 dan M3-12/M4-06/07/08 bergantung pada API yang sudah stabil; jangan menunda UI sampai M6.

### Dependensi lintas milestone yang mudah terlewat

- M3-12 (login SPA ↔ authorize) butuh M2-07 dan M3-05.
- M4-02 (forced MFA) butuh session middleware M2-02 dan TOTP M4-01.
- M4-05 settings butuh enkripsi SMTP dari M1-08 dan master key M3-02.
- M4.5-01 (Playwright smoke) harus selesai sebelum semua perubahan UI; suite yang sama dipakai sebagai gate di setiap commit M4.5.
- M4.5-05/06 mengubah markup yang diuji smoke; jalankan suite setelah tiap commit, bukan hanya di akhir.
- M5-05 rotasi key butuh M3-02 dan skema key M1-08.
- M5-07 memperluas suite M4.5-01, bukan membuat suite baru.
- M6-01 conformance butuh discovery, prompt/max_age, logout, dan refresh dari M3; jalankan setelah M4.5 stabil.

---

## 5. Gate Verifikasi per Milestone

| Milestone | Gate otomatis | Gate manual |
|---|---|---|
| M1 | CI, unit, migrasi up/down, health | `make dev` dari clone bersih |
| M2 | Integration auth, email outbox | Login + invite + reset via MailHog |
| M3 | Integration OIDC, RP e2e | Code flow + refresh + logout dengan RP nyata |
| M4 | Admin API + MFA test | CRUD admin, forced MFA, last-admin |
| M4.5 | Playwright smoke, typecheck/lint/build, `go build` | Browser pass semua halaman desktop + mobile; keyboard nav dan focus trap |
| M5 | Security test, Redis-down, load test | Checklist keamanan ARCHITECTURE §10 |
| M6 | Conformance, e2e penuh | Backup/restore drill + sign-off PRD §13 |

---

## 6. Risiko Jadwal & Aturan Re-scope

| Risiko | Tanda awal | Tindakan |
|---|---|---|
| Adapter `zitadel/oidc` lebih sulit dari estimasi | M3-03/06 melewati 12 jam | Timebox 1 hari; jika macet, pakai implementasi manual minimal untuk code flow dan kurangi fitur opsional (client credentials digeser M5) |
| Conformance menemukan banyak temuan | M6-01 melewati 6 jam | Pakai buffer; geser rilis 1 minggu; jangan menurunkan standar |
| Libur akhir tahun | Minggu 7–8 kapasitas turun | Mulai lebih awal atau geser 1 minggu; buffer bukan untuk fitur |
| Argon2 tidak mencapai p95 | M5-06 gagal target | Turunkan parameter ke 32 MiB atau batasi konkurensi; catat trade-off |
| UI admin melebar | M4-06/07 melewati 11 jam | Fokus layar users + clients; roles/groups read-only di v1.0 |
| Upgrade Tailwind 4 mengubah token/utility | Halaman bergeser atau ada class hilang | Playwright smoke + screenshot baseline; commit Tailwind terpisah dari perubahan markup |
| API `shadcn-vue` 2.9 berbeda dari 1.x (`data-slot`, style baru) | Typecheck/lint gagal massal | Timebox 1 hari; fallback `shadcn-vue@1.0.3` + Tailwind 3 dengan revert M4.5-02 |
| `reka-ui` Select menolak `value=""` | Filter "semua" rusak | Sentinel `all` yang di-map ke string kosong |
| `Input.focus()` hilang setelah regenerasi | Auto-focus kode OTP/MFA rusak | Pertahankan `defineExpose` di wrapper Input |
| Bundle naik karena `lucide-vue-next` | First load melambat | Import tree-shaken; cek ukuran bundle di gate M4.5 |
| Redis mati saat uji | M5-04 gagal | Perbaiki fallback; jangan rilis sebelum matriks degradasi hijau |

**Aturan re-scope:** task P0 tidak boleh dihapus tanpa revisi PRD. P1 boleh dipotong kapan saja. Jika satu task melewati estimasi >4 jam, hentikan dan pecah ulang atau pangkas output.

---

## 7. Backlog P1/P2 (tidak dijadwalkan)

P1 hanya dikerjakan jika buffer >10 jam dan milestone berikutnya belum dimulai.

| ID | Tugas | Jam | Dampak jika ditunda |
|---|---|---:|---|
| P1-01 | Import user CSV (FR-ADM-06) | 4 | Admin input manual |
| P1-02 | Key viewer metadata (FR-ADM-07) | 2 | Cek key via CLI |
| P1-03 | Export audit CSV (FR-AUD-04) | 2 | Query DB manual |
| P1-04 | Rehash password saat parameter berubah (FR-USR-11) | 1 | User lama tetap hash lama |
| P1-05 | Remember-me per perangkat (FR-AUT-11) | 2 | Revoke semua sesi |
| P1-06 | TTL token per client (FR-OID-13) | 2 | TTL global |
| P1-07 | Riwayat rotasi secret (FR-CLI-04) | 2 | Hanya metadata saat ini |
| P1-08 | Interface KeyStore untuk KMS (FR-KEY-03) | 2 | Refactor saat pindah KMS |
| P1-09 | Toast feedback (Sonner) di seluruh mutasi admin | 2 | Feedback masih inline `<Alert>` |

P2 (realm CRUD, email change, role inheritance via group, device code, WebAuthn, SAML, Kafka) tidak masuk backlog 8 minggu.

---

## 8. Rencana Harian 3 Hari Pertama

| Hari | Fokus | Output |
|---|---|---|
| Hari 1 | P0-01, M1-01, M1-02 | Repo init, Makefile, config loader |
| Hari 2 | M1-03, M1-04 | Logger + tracing context + test |
| Hari 3 | M1-05, M1-06, M1-07 | Postgres + Redis + migrasi batch 1 |

Setelah hari ke-3, jalankan demo internal: `make dev` menyalakan Postgres/Redis, migrasi batch 1 jalan, logger mencetak `trace_id`/`span_id`. Ini baseline untuk semua task berikutnya.

---

## 9. Change Control

- Plan direview setiap akhir minggu; selisih estimasi dicatat.
- Perubahan scope hanya lewat revisi `docs/PRD.md`; task baru harus punya ID dan estimasi.
- Jika timeline bergeser >3 hari, perbarui tabel §2 dan kabari pemilik produk.
- Revisi v1.1 menyesuaikan §2, §3 (M4.5), §4, §5, §6, dan §7. Keputusan Tailwind 4 + shadcn-vue dicatat di `ARCHITECTURE` (bagian frontend) dan `README`.
- Dokumentasi yang wajib ikut berubah saat implementasi: `api/openapi.yaml` (kontrak), `ARCHITECTURE` (keputusan teknis), `README` (stack UI dan cara menjalankan), `RUNBOOK` (operasional).
