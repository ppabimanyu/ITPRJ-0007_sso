export default {
  app: {
    name: 'SSO Service',
    footer: 'Identitas internal untuk setiap aplikasi.',
  },
  nav: {
    home: 'Beranda',
    admin: 'Admin',
  },
  home: {
    eyebrow: 'Kerangka Milestone 1',
    description:
      'Layanan single sign-on untuk aplikasi internal. Kerangka ini menguji Vue, router, query, i18n, dan design system terhadap API Go yang berjalan.',
    api: {
      title: 'Status API',
      description: 'Halaman beranda membaca endpoint versi dengan fallback health check.',
      checking: 'Memeriksa',
      ready: 'Siap',
      unavailable: 'Tidak tersedia',
      retry: 'Periksa lagi',
      source: 'Sumber',
      version: 'Versi',
      commit: 'Commit',
      goVersion: 'Go',
      fallback: 'Endpoint versi tidak tersedia. Health check proses menjawab sebagai gantinya.',
    },
    stack: {
      title: 'Stack frontend',
      description: 'Bagian yang diperluas M2 dan M4 tanpa diganti.',
    },
  },
  admin: {
    title: 'Konsol admin',
    description:
      'Halaman user, client, role, group, sesi, audit, dan settings admin hadir di milestone 4.',
    backHome: 'Kembali ke beranda',
  },
  notFound: {
    title: 'Halaman tidak ditemukan',
    description: 'Rute ini belum ada. Mungkin hadir di milestone berikutnya.',
    backHome: 'Kembali ke beranda',
  },
}
