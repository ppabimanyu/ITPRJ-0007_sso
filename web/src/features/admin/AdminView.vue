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
  <div class="grid gap-6 lg:grid-cols-[210px_minmax(0,1fr)] lg:items-start">
    <aside class="min-w-0 lg:sticky lg:top-24">
      <p class="kicker mb-2 hidden lg:block">
        {{ t('admin.nav.label') }}
      </p>
      <nav
        class="flex w-full min-w-0 gap-1 overflow-x-auto pb-2 lg:flex-col lg:gap-0.5 lg:overflow-visible lg:pb-0"
        :aria-label="t('admin.nav.label')"
      >
        <RouterLink
          v-for="link in links"
          :key="link.to"
          :to="link.to"
          class="admin-link whitespace-nowrap"
          :class="{ 'admin-link-active': isActive(link.to) }"
        >
          {{ link.label }}
        </RouterLink>
      </nav>
    </aside>

    <div class="min-w-0 space-y-6">
      <header class="space-y-2 border-b border-border/70 pb-5">
        <p class="kicker">
          {{ t('nav.admin') }}
        </p>
        <h1 class="font-display text-3xl sm:text-4xl">
          {{ t('admin.title') }}
        </h1>
        <p class="max-w-2xl text-muted-foreground">
          {{ t('admin.description') }}
        </p>
      </header>

      <RouterView />
    </div>
  </div>
</template>
