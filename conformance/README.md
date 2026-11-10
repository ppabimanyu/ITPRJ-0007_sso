# OpenID conformance

Status: suite lokal sudah dijalankan pada 10 November 2026. Happy flow Basic OP
lulus setelah perbaikan PKCE. Seluruh 35 modul belum hijau karena upload
screenshot pada tes `prompt=login` belum diotomatiskan.

Environment ini tidak punya endpoint publik atau akun suite hosted. Dokumen ini
mencatat langkah lokal dan hasil yang sudah ada.

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

## Hasil lokal, 10 November 2026

Suite prebuilt `latest` (versi 5.3.4) berjalan dengan `docker-compose-prebuilt.yml`.
OP berjalan di `http://host.docker.internal:8093` dengan `EXTERNAL_URL` yang sama,
sehingga issuer discovery cocok dengan URL yang dipakai suite. Browser memakai
resolver `host.docker.internal` dan `localhost.emobix.co.uk` ke `127.0.0.1`.

Plan `oidcc-basic-certification-test-plan`, alias `conformance-basic`, static
client confidential, discovery.

- `oidcc-server`: PASS setelah PKCE dilonggarkan untuk confidential client.
- `oidcc-userinfo-get`, `oidcc-userinfo-post-header`: PASS.
- `oidcc-userinfo-post-body`: PASS setelah userinfo menerima `access_token` di
  form body.
- `oidcc-ensure-request-without-nonce-succeeds-for-code-flow`: PASS.
- `oidcc-prompt-none-not-logged-in`: mengembalikan `login_required`, sesuai.
- `oidcc-scope-profile`: WARNING karena klaim profil opsional seperti
  `website`, `birthdate`, dan `preferred_username` tidak ada di model user.
- `oidcc-scope-email`: WARNING karena `email` juga muncul di ID token, bukan
  hanya userinfo.
- `oidcc-prompt-login`: mencapai otorisasi kedua dan menunggu upload screenshot.
  Ini langkah UI suite, bukan kegagalan OP.

Sisa pekerjaan sebelum full plan hijau: otomatiskan upload screenshot dan
jalankan 35 modul berurutan dengan alias yang sama tanpa konflik.
