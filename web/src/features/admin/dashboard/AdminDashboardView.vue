<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuery } from '@tanstack/vue-query'

import { Badge } from '@/components/ui/badge'

import { listAuditEvents, listClients, listGroups, listRoles, listAdminSessions, listUsers } from '@/lib/admin/api'
import { adminKeys } from '@/lib/query-keys'

import AdminProblem from '../components/AdminProblem.vue'
import { useAdminFormat } from '../use-admin-format'

const { t } = useI18n()
const { formatDate } = useAdminFormat()

const users = useQuery({
  queryKey: computed(() => adminKeys.userList({ limit: 5 })),
  queryFn: () => listUsers({ limit: 5 }),
})

const clients = useQuery({
  queryKey: computed(() => adminKeys.clientList({ limit: 5 })),
  queryFn: () => listClients({ limit: 5 }),
})

const sessions = useQuery({
  queryKey: computed(() => adminKeys.sessionList({ status: 'active', limit: 5 })),
  queryFn: () => listAdminSessions({ status: 'active', limit: 5 }),
})

const roles = useQuery({
  queryKey: adminKeys.roles,
  queryFn: listRoles,
})

const groups = useQuery({
  queryKey: adminKeys.groups,
  queryFn: listGroups,
})

const audit = useQuery({
  queryKey: computed(() => adminKeys.auditList({ limit: 5 })),
  queryFn: () => listAuditEvents({ limit: 5 }),
})

const userCount = computed(() => pageCount(users.data.value))
const clientCount = computed(() => pageCount(clients.data.value))
const sessionCount = computed(() => pageCount(sessions.data.value))
const roleCount = computed(() => (roles.data.value ? String(roles.data.value.length) : '…'))
const groupCount = computed(() => (groups.data.value ? String(groups.data.value.length) : '…'))

const register = computed(() => [
  {
    to: '/admin/users',
    label: t('admin.dashboard.users'),
    hint: t('admin.dashboard.openUsers'),
    count: userCount.value,
    numeric: true,
  },
  {
    to: '/admin/clients',
    label: t('admin.dashboard.clients'),
    hint: t('admin.dashboard.openClients'),
    count: clientCount.value,
    numeric: true,
  },
  {
    to: '/admin/roles',
    label: t('admin.dashboard.roles'),
    hint: t('admin.dashboard.openRoles'),
    count: roleCount.value,
    numeric: true,
  },
  {
    to: '/admin/groups',
    label: t('admin.dashboard.groups'),
    hint: t('admin.dashboard.openGroups'),
    count: groupCount.value,
    numeric: true,
  },
  {
    to: '/admin/sessions',
    label: t('admin.dashboard.sessions'),
    hint: t('admin.dashboard.openSessions'),
    count: sessionCount.value,
    numeric: true,
  },
  {
    to: '/admin/settings',
    label: t('admin.dashboard.settings'),
    hint: t('admin.dashboard.settingsHint'),
    count: t('admin.common.open'),
    numeric: false,
  },
])

function pageCount(page: { items: unknown[]; nextAfter: string | null } | undefined): string {
  if (!page) {
    return '…'
  }
  return page.nextAfter === null ? String(page.items.length) : `${page.items.length}+`
}
</script>

<template>
  <div class="space-y-8">
    <section class="overflow-hidden rounded-lg border bg-card">
      <div class="border-b border-border/70 px-5 py-4">
        <h2 class="font-display text-xl">
          {{ t('admin.dashboard.registerTitle') }}
        </h2>
        <p class="text-sm text-muted-foreground">
          {{ t('admin.dashboard.registerDescription') }}
        </p>
      </div>
      <div class="divide-y divide-border/70">
        <RouterLink
          v-for="row in register"
          :key="row.to"
          :to="row.to"
          class="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-4 px-5 py-4 transition-colors hover:bg-secondary/40"
        >
          <div class="min-w-0">
            <p class="font-medium">
              {{ row.label }}
            </p>
            <p class="text-xs text-muted-foreground">
              {{ row.hint }}
            </p>
          </div>
          <span
            :class="
              row.numeric
                ? 'font-display text-2xl tabular-nums'
                : 'text-xs font-medium text-muted-foreground'
            "
          >
            {{ row.count }}
          </span>
        </RouterLink>
      </div>
    </section>

    <section class="space-y-3">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <h2 class="font-display text-2xl">
          {{ t('admin.dashboard.recentAudit') }}
        </h2>
        <RouterLink
          to="/admin/audit"
          class="text-sm font-medium text-primary underline-offset-4 hover:underline"
        >
          {{ t('admin.dashboard.viewAllAudit') }}
        </RouterLink>
      </div>

      <AdminProblem :error="audit.error.value" />

      <p
        v-if="audit.isPending.value"
        class="text-sm text-muted-foreground"
      >
        {{ t('common.loading') }}
      </p>

      <p
        v-else-if="audit.data.value && audit.data.value.items.length === 0"
        class="rounded-lg border border-dashed px-4 py-6 text-center text-sm text-muted-foreground"
      >
        {{ t('admin.dashboard.emptyAudit') }}
      </p>

      <div
        v-else-if="audit.data.value"
        class="overflow-hidden rounded-lg border bg-card"
      >
        <ul class="divide-y divide-border/70">
          <li
            v-for="event in audit.data.value.items"
            :key="event.id"
            class="grid gap-2 px-5 py-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center"
          >
            <div class="flex flex-wrap items-center gap-2">
              <Badge variant="outline">
                {{ event.type }}
              </Badge>
              <span class="text-xs text-muted-foreground">{{ event.actorType }}</span>
            </div>
            <span class="text-xs text-muted-foreground">{{ formatDate(event.createdAt) }}</span>
          </li>
        </ul>
      </div>
    </section>
  </div>
</template>
