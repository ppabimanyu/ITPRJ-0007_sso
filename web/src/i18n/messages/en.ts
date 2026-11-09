export default {
  app: {
    name: 'SSO Service',
    footer: 'Internal identity for every application.',
  },
  nav: {
    home: 'Home',
    admin: 'Admin',
  },
  home: {
    eyebrow: 'Milestone 1 shell',
    description:
      'A single sign-on service for internal applications. This shell exercises the Vue, router, query, i18n, and design-system stack against the live Go API.',
    api: {
      title: 'API health',
      description: 'The home page reads the version endpoint with a health check fallback.',
      checking: 'Checking',
      ready: 'Ready',
      unavailable: 'Unavailable',
      retry: 'Check again',
      source: 'Source',
      version: 'Version',
      commit: 'Commit',
      goVersion: 'Go',
      fallback: 'Version endpoint unavailable. The process health check answered instead.',
    },
    stack: {
      title: 'Frontend stack',
      description: 'The pieces M2 and M4 extend without replacing.',
    },
  },
  admin: {
    title: 'Admin console',
    description:
      'Admin user, client, role, group, session, audit, and settings screens land in milestone 4.',
    backHome: 'Back to home',
  },
  notFound: {
    title: 'Page not found',
    description: 'The route does not exist yet. It may arrive in a later milestone.',
    backHome: 'Back to home',
  },
}
