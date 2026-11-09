<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuery } from '@tanstack/vue-query'

import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

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

function pageCount(page: { items: unknown[]; nextAfter: string | null } | undefined): string {
  if (!page) {
    return '…'
  }
  return page.nextAfter === null ? String(page.items.length) : `${page.items.length}+`
}
</script>

<template>
  <div class="space-y-6">
    <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      <Card>
        <CardHeader class="pb-2">
          <CardDescription>{{ t('admin.dashboard.users') }}</CardDescription>
          <CardTitle class="text-3xl">
            {{ userCount }}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <RouterLink
            to="/admin/users"
            class="text-sm font-medium text-primary underline-offset-4 hover:underline"
          >
            {{ t('admin.dashboard.openUsers') }}
          </RouterLink>
        </CardContent>
      </Card>
      <Card>
        <CardHeader class="pb-2">
          <CardDescription>{{ t('admin.dashboard.clients') }}</CardDescription>
          <CardTitle class="text-3xl">
            {{ clientCount }}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <RouterLink
            to="/admin/clients"
            class="text-sm font-medium text-primary underline-offset-4 hover:underline"
          >
            {{ t('admin.dashboard.openClients') }}
          </RouterLink>
        </CardContent>
      </Card>
      <Card>
        <CardHeader class="pb-2">
          <CardDescription>{{ t('admin.dashboard.sessions') }}</CardDescription>
          <CardTitle class="text-3xl">
            {{ sessionCount }}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <RouterLink
            to="/admin/sessions"
            class="text-sm font-medium text-primary underline-offset-4 hover:underline"
          >
            {{ t('admin.dashboard.openSessions') }}
          </RouterLink>
        </CardContent>
      </Card>
      <Card>
        <CardHeader class="pb-2">
          <CardDescription>{{ t('admin.dashboard.roles') }}</CardDescription>
          <CardTitle class="text-3xl">
            {{ roleCount }}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <RouterLink
            to="/admin/roles"
            class="text-sm font-medium text-primary underline-offset-4 hover:underline"
          >
            {{ t('admin.dashboard.openRoles') }}
          </RouterLink>
        </CardContent>
      </Card>
      <Card>
        <CardHeader class="pb-2">
          <CardDescription>{{ t('admin.dashboard.groups') }}</CardDescription>
          <CardTitle class="text-3xl">
            {{ groupCount }}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <RouterLink
            to="/admin/groups"
            class="text-sm font-medium text-primary underline-offset-4 hover:underline"
          >
            {{ t('admin.dashboard.openGroups') }}
          </RouterLink>
        </CardContent>
      </Card>
      <Card>
        <CardHeader class="pb-2">
          <CardDescription>{{ t('admin.dashboard.settings') }}</CardDescription>
          <CardTitle class="text-lg">
            {{ t('admin.dashboard.settingsHint') }}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <RouterLink
            to="/admin/settings"
            class="text-sm font-medium text-primary underline-offset-4 hover:underline"
          >
            {{ t('admin.dashboard.openSettings') }}
          </RouterLink>
        </CardContent>
      </Card>
    </div>

    <div class="space-y-3">
      <div class="flex items-center justify-between gap-3">
        <h2 class="text-lg font-semibold tracking-tight">
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

      <ul
        v-else-if="audit.data.value"
        class="divide-y rounded-lg border bg-card"
      >
        <li
          v-for="event in audit.data.value.items"
          :key="event.id"
          class="flex flex-col gap-1 px-4 py-3 sm:flex-row sm:items-center sm:justify-between"
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
  </div>
</template>
