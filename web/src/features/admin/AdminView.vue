<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'

const { t } = useI18n()
const route = useRoute()

interface AdminLink {
  to: string
  label: string
}

const links = computed<AdminLink[]>(() => [
  { to: '/admin', label: t('admin.nav.dashboard') },
  { to: '/admin/users', label: t('admin.nav.users') },
  { to: '/admin/clients', label: t('admin.nav.clients') },
  { to: '/admin/roles', label: t('admin.nav.roles') },
  { to: '/admin/groups', label: t('admin.nav.groups') },
  { to: '/admin/sessions', label: t('admin.nav.sessions') },
  { to: '/admin/audit', label: t('admin.nav.audit') },
  { to: '/admin/settings', label: t('admin.nav.settings') },
])

function isActive(to: string): boolean {
  if (to === '/admin') {
    return route.path === '/admin'
  }
  return route.path === to || route.path.startsWith(`${to}/`)
}
</script>

<template>
  <section class="space-y-8">
    <div class="space-y-3">
      <p class="text-xs font-medium uppercase tracking-[0.2em] text-muted-foreground">
        {{ t('nav.admin') }}
      </p>
      <h1 class="text-3xl font-semibold tracking-tight sm:text-4xl">
        {{ t('admin.title') }}
      </h1>
      <p class="max-w-3xl text-muted-foreground">
        {{ t('admin.description') }}
      </p>
    </div>

    <nav
      class="flex flex-wrap gap-1 rounded-lg border bg-card p-1"
      :aria-label="t('admin.nav.label')"
    >
      <RouterLink
        v-for="link in links"
        :key="link.to"
        :to="link.to"
        class="rounded-md px-3 py-2 text-sm transition-colors"
        :class="
          isActive(link.to)
            ? 'bg-primary text-primary-foreground'
            : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
        "
      >
        {{ link.label }}
      </RouterLink>
    </nav>

    <RouterView />
  </section>
</template>
