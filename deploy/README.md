# Deploy produksi

Berkas di folder ini adalah template. Deploy sebenarnya butuh host, domain,
kredensial SMTP, dan master key dari operator.

## 1. Siapkan env

```sh
cp deploy/.env.example deploy/.env
```

Isi `deploy/.env`. `POSTGRES_PASSWORD` dan `REDIS_PASSWORD` harus sama dengan
password di `DATABASE_URL` dan `REDIS_URL`. Buat master key 32 byte:

```sh
openssl rand -base64 32
```

Letakkan hasilnya di `SSO_MASTER_KEYS` dengan id tanggal, misalnya
`{"2026-11":"<base64>"}`, dan set `SSO_MASTER_KEY_ID=2026-11`.

Set `TRUSTED_PROXY_CIDRS` ke CIDR network Docker yang berisi Caddy. Tanpa itu,
rate limit melihat semua trafik sebagai satu IP proxy.

## 2. Build image

```sh
docker build -t sso-service:$(git rev-parse --short HEAD) .
```

Set `SSO_IMAGE` di `deploy/.env` ke tag yang sama.

## 3. Migrasi dan bootstrap

```sh
docker compose -f deploy/compose.prod.yml --env-file deploy/.env run --rm migrate
printf '%s\n' "$SSO_ADMIN_PASSWORD" | docker compose -f deploy/compose.prod.yml \
  --env-file deploy/.env run --rm -T migrate bootstrap \
  --email admin@example.com --password-stdin
```

Bootstrap hanya diperlukan saat realm dan admin pertama belum ada.

## 4. Jalankan

```sh
docker compose -f deploy/compose.prod.yml --env-file deploy/.env up -d
curl -fsS https://sso.example.com/healthz
curl -fsS https://sso.example.com/readyz
```

Caddy mengambil sertifikat TLS otomatis untuk `SSO_DOMAIN`. Port 80 dan 443
harus terbuka dan domain harus mengarah ke host ini.

## 5. Backup

Pasang cron harian dari `docs/RUNBOOK.md`. Simpan file terenkripsi di lokasi
terpisah dari host database, dan uji restore minimal sekali per rilis.

## 6. Catatan

- Jangan membuka port Postgres atau Redis ke publik. Compose ini tidak
  memetakan port keduanya.
- Satu instance app untuk v1. Menambah instance membutuhkan advisory lock untuk
  worker outbox dan retensi.
- Rollback kode mengikuti `docs/RUNBOOK.md` bagian 10.
