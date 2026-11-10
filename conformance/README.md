# OpenID conformance

Status: belum dijalankan. Environment ini tidak punya endpoint publik, akun
suite, atau sesi browser untuk menyelesaikan test plan. Dokumen ini mencatat
langkah yang harus dijalankan sebelum rilis.

## Prasyarat

- Docker dengan Compose v2.
- OP yang bisa dijangkau suite. Di mesin yang sama, gunakan
  `http://host.docker.internal:8093` dari container suite. Untuk suite hosted,
  OP harus punya URL HTTPS publik.
- Browser untuk login saat test plan meminta autentikasi.
- Client statis di OP untuk suite.

## 1. Jalankan suite

```sh
git clone https://gitlab.com/openid/conformance-suite.git
cd conformance-suite
git checkout release-v5.2.4
IMAGE_TAG=release-v5.2.4 docker compose -f docker-compose-prebuilt.yml pull
IMAGE_TAG=release-v5.2.4 docker compose -f docker-compose-prebuilt.yml up -d
```

Buka `https://localhost:8443/` dan terima sertifikat self-signed.

## 2. Siapkan OP

Jalankan stack lokal pada port yang bisa dijangkau suite.

```sh
E2E_PORT=8093 bash scripts/e2e-stack.sh
```

Buat client confidential di admin console dengan redirect
`https://localhost:8443/test/a/conformance-basic/callback` dan grant
`authorization_code`, `refresh_token`. Simpan client ID dan secret.

Discovery URL untuk suite adalah
`http://host.docker.internal:8093/realms/default/.well-known/openid-configuration`.

## 3. Jalankan plan

Di UI suite, buat plan **OpenID Connect Core: Basic Certification Profile
Authorization server test**. Isi client statis dengan client ID dan secret dari
langkah 2, lalu mulai plan.

Saat plan membuka halaman login, masuk dengan user seed `user@example.com`.
Selesaikan seluruh modul. Unduh hasil dan simpan ringkasannya di
`docs/RELEASE_CHECKLIST.md`.

## 4. Ekspektasi

Semua modul Basic OP harus hijau atau hanya berbeda pada warning yang
didokumentasikan. Error kritis berarti belum memenuhi PRD §13 butir 2.
