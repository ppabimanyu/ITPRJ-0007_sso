# Runbook Operasional

Panduan ini untuk operator yang menjalankan SSO Service. Semua perintah
dijalankan dari root repositori kecuali disebut lain. Template deploy produksi
ada di `deploy/`.

## 1. Prasyarat

- Go 1.27 dan Node.js 22 untuk build, atau Docker untuk image multi-stage.
- Postgres 16 dan Redis 7.
- `SSO_MASTER_KEYS` berisi key aktif dan `SSO_MASTER_KEY_ID` menunjuk key itu.
- `APP_ENV=production` agar cookie memakai prefiks `__Host-` dan atribut
  `Secure`, serta HSTS aktif.

## 2. Deploy

Build image dari `Dockerfile`. Image berisi `/server` dan `/migration` di atas
distroless, berjalan sebagai `nonroot`.

```sh
docker build -t sso-service:$(git rev-parse --short HEAD) .
```

Jalankan migrasi sebagai langkah terpisah sebelum app start.

```sh
docker run --rm --env-file /etc/sso/production.env \
  --entrypoint /migration sso-service:<tag> up
```

Bootstrap pertama kali hanya jika realm dan admin belum ada. Password dibaca
dari stdin, tidak pernah dari argumen.

```sh
printf '%s\n' "$SSO_ADMIN_PASSWORD" | docker run --rm -i \
  --env-file /etc/sso/production.env --entrypoint /migration \
  sso-service:<tag> bootstrap --email admin@example.com --password-stdin
```

Jalankan app di belakang reverse proxy TLS. Caddy contoh ada di `Caddyfile`
untuk pengembangan. Untuk produksi, ganti alamat menjadi domain nyata dan
teruskan header `X-Forwarded-For`. Isi `TRUSTED_PROXY_CIDRS` dengan alamat
proxy agar `RealIP` hanya percaya header dari proxy.

Verifikasi setelah start.

```sh
curl -fsS https://sso.example.com/healthz
curl -fsS https://sso.example.com/readyz
```

`/readyz` mengembalikan `200` dengan `status: ready` saat semua sehat, dan
`200` dengan `status: degraded` saat Redis mati. Status `unready` dengan `503`
hanya terjadi saat database atau versi migrasi bermasalah.

## 3. Sinyal dan log

- Setiap request menulis satu baris JSON `http_request` dengan `request_id`,
  `trace_id`, `span_id`, `route`, `status`, dan `duration_ms`.
- `user_id` ditambahkan setelah sesi terautentikasi.
- Logger meredaksi nilai yang terlihat seperti token atau secret. Jangan
  menaruh email atau token di metadata log.
- Endpoint metrik Prometheus belum tersedia. Pantau lewat log dan endpoint
  health sampai metrik ditambahkan.

## 4. Backup

Backup harian dengan `pg_dump` format custom, lalu enkripsi sebelum disimpan di
luar host database. Redis tidak di-backup.

```sh
stamp=$(date -u +%Y%m%dT%H%M%SZ)
docker compose -f deploy/compose.prod.yml --env-file deploy/.env exec -T postgres \
  pg_dump -U "${POSTGRES_USER:-sso}" -Fc "${POSTGRES_DB:-sso}" > "/backup/sso-$stamp.dump"
age -r "$BACKUP_RECIPIENT" -o "/backup/sso-$stamp.dump.age" "/backup/sso-$stamp.dump"
rm "/backup/sso-$stamp.dump"
find /backup -name 'sso-*.dump.age' -mtime +7 -delete
```

Simpan salinan di lokasi terpisah dari database. Uji restore minimal sekali
per rilis, bukan hanya saat insiden.

Backup harus disimpan bersama master key yang mengenkripsi isinya. Tanpa key
yang cocok, signing key dan secret MFA di backup tidak bisa dibaca setelah
restore. Jika `age` tidak tersedia, `openssl enc -aes-256-cbc -pbkdf2 -salt`
dengan passphrase file adalah fallback yang dipakai pada drill lokal.

## 5. Restore

Restore ke database kosong, lalu jalankan migrasi untuk memastikan versi
skema terbaru. Jangan restore ke database yang sedang dipakai app.

```sh
RESTORE_URL="postgres://sso:<password>@postgres:5432/sso_restore?sslmode=disable"
age -d -i "$BACKUP_IDENTITY" -o /tmp/sso.dump /backup/sso-<stamp>.dump.age
docker compose -f deploy/compose.prod.yml --env-file deploy/.env exec -T postgres \
  psql -U "${POSTGRES_USER:-sso}" -d postgres -c 'CREATE DATABASE sso_restore OWNER sso'
docker compose -f deploy/compose.prod.yml --env-file deploy/.env exec -T postgres \
  pg_restore -U "${POSTGRES_USER:-sso}" -d sso_restore --no-owner --no-privileges < /tmp/sso.dump
docker compose -f deploy/compose.prod.yml --env-file deploy/.env run --rm \
  -e DATABASE_URL="$RESTORE_URL" migrate
```

Verifikasi jumlah baris dan versi migrasi, lalu arahkan app ke database hasil
restore dan cek `/readyz` serta login admin.

```sh
docker compose -f deploy/compose.prod.yml --env-file deploy/.env exec -T postgres \
  psql -U "${POSTGRES_USER:-sso}" -d sso_restore -c 'SELECT count(*) FROM "user"'
docker compose -f deploy/compose.prod.yml --env-file deploy/.env exec -T postgres \
  psql -U "${POSTGRES_USER:-sso}" -d sso_restore -c 'SELECT max(version_id) FROM goose_db_version'
```

## 6. Rotasi signing key

```sh
docker run --rm --env-file /etc/sso/production.env \
  --entrypoint /migration sso-service:<tag> rotate-key
```

Perintah membuat key aktif baru dan mempensiunkan key lama setelah 24 jam.
Token yang ditandatangani sebelum rotasi tetap valid selama key lama masih
dipublikasikan di JWKS. Verifikasi `kid` baru muncul di
`/realms/{realm}/jwks`.

## 7. Rotasi master key

Urutannya penting agar tidak ada downtime.

1. Tambahkan key baru ke `SSO_MASTER_KEYS` dan arahkan `SSO_MASTER_KEY_ID` ke
   key baru, sambil tetap menyimpan key lama.
2. Deploy app dengan keyring baru. App masih bisa membaca ciphertext lama.
3. Jalankan re-enkripsi.

   ```sh
   docker run --rm --env-file /etc/sso/production.env \
     --entrypoint /migration sso-service:<tag> rotate-master-key
   ```

4. Tunggu sampai perintah melaporkan `signing_keys=0 smtp_passwords=0
   mfa_secrets=0` pada run berikutnya.
5. Hapus key lama dari `SSO_MASTER_KEYS` dan deploy ulang.

Perintah berjalan dalam satu transaksi, mengunci baris yang disentuh, dan
melewati baris yang sudah memakai key aktif. Timeout perintah 30 menit.

## 8. Cabut sesi

Gunakan admin console atau API. Cabut per sesi lebih aman daripada SQL manual
karena refresh family ikut dicabut.

```sh
curl -X DELETE https://sso.example.com/api/v1/sessions/<session-id> \
  -H "X-CSRF-Token: $CSRF" -H "Origin: https://sso.example.com" \
  -b cookies.txt
```

Untuk insiden yang mencakup satu user, nonaktifkan akun lewat admin console.
Itu mencabut semua sesi dan refresh token user tersebut.

## 9. Redis mati

Perilaku yang diharapkan.

- Sesi yang sudah ada, `/api/v1/auth/session`, `/api/v1/me`, logout, JWKS, dan
  discovery tetap berjalan dari Postgres dan key material.
- Login baru, CSRF pra-sesi, dan endpoint kredensial OIDC mengembalikan `503`
  terkontrol.
- `/readyz` mengembalikan `200` dengan `status: degraded`.
- Setelah Redis kembali, login dan endpoint kredensial pulih tanpa restart.

Jangan restart app hanya karena Redis mati. Sesi ada di Postgres, jadi restart
tidak diperlukan dan hanya menambah gangguan.

## 10. Rollback

- Simpan image sebelumnya dengan tag commit.
- Migrasi bersifat maju. Rollback kode tanpa rollback skema hanya aman jika
  rilis baru tidak menambah kolom yang dibutuhkan kode lama; periksa migrasi
  yang berubah sebelum rollback.
- Untuk rollback penuh, jalankan satu langkah `down` dengan image saat ini,
  lalu jalankan image sebelumnya.

  ```sh
  docker run --rm --env-file /etc/sso/production.env \
    --entrypoint /migration sso-service:<tag> down
  ```

## 11. Catatan drill

Catat tanggal, operator, perintah, dan hasil di `docs/RELEASE_CHECKLIST.md`.
Drill lokal tidak menggantikan drill staging.
