<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuery } from '@tanstack/vue-query'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

import { listAdminSessions, listUsers } from '@/lib/admin/api'
import type { AdminSession } from '@/lib/admin/types'
import { adminKeys } from '@/lib/query-keys'

import AdminConfirmDialog from '../components/AdminConfirmDialog.vue'
import AdminPager from '../components/AdminPager.vue'
import AdminProblem from '../components/AdminProblem.vue'
import { useRevokeAdminSession } from '../queries'
import { useAdminFormat } from '../use-admin-format'
import { useAdminPager } from '../use-admin-pager'

type SessionStatus = '' | 'active' | 'revoked' | 'expired'

const { t } = useI18n()
const { formatCompactDate } = useAdminFormat()
const pager = useAdminPager()

const userId = ref('')
const status = ref<SessionStatus>('')
const revokeTarget = ref<AdminSession | null>(null)

const usersQuery = useQuery({
  queryKey: computed(() => adminKeys.userList({ limit: 100 })),
  queryFn: () => listUsers({ limit: 100 }),
})

const params = computed(() => ({
  userId: userId.value === '' ? undefined : userId.value,
  status: status.value === '' ? undefined : status.value,
  after: pager.after.value ?? undefined,
  limit: 25,
}))

const { data, isPending, isFetching, error } = useQuery({
  queryKey: computed(() => adminKeys.sessionList(params.value)),
  queryFn: () => listAdminSessions(params.value),
})

const revokeMutation = useRevokeAdminSession()
const sessions = computed(() => data.value?.items ?? [])
const users = computed(() => usersQuery.data.value?.items ?? [])

watch([userId, status], () => {
  pager.reset()
})

function setUserId(value: unknown) {
  userId.value = String(value)
}

function setStatus(value: unknown) {
  status.value = (value === 'all' ? '' : value) as SessionStatus
}

function sessionStatus(session: AdminSession): 'active' | 'revoked' | 'expired' {
  if (session.revokedAt !== null) {
    return 'revoked'
  }
  return new Date(session.expiresAt).getTime() <= Date.now() ? 'expired' : 'active'
}

function statusVariant(value: 'active' | 'revoked' | 'expired'): 'secondary' | 'destructive' | 'outline' {
  if (value === 'revoked') {
    return 'destructive'
  }
  return value === 'active' ? 'secondary' : 'outline'
}

const browserPatterns: Array<[RegExp, string]> = [
  [/Edg\//, 'Edge'],
  [/OPR\//, 'Opera'],
  [/Chrome\//, 'Chrome'],
  [/Firefox\//, 'Firefox'],
  [/Version\/[\d.]+.*Safari\//, 'Safari'],
  [/curl\//, 'curl'],
]

const systemPatterns: Array<[RegExp, string]> = [
  [/iPhone|iPad|iPod/, 'iOS'],
  [/Android/, 'Android'],
  [/Mac OS X/, 'macOS'],
  [/Windows/, 'Windows'],
  [/CrOS/, 'ChromeOS'],
  [/Linux/, 'Linux'],
]

function deviceName(userAgent: string | null): string {
  if (userAgent === null || userAgent === '') {
    return t('admin.sessions.unknownDevice')
  }
  const browser = browserPatterns.find(([pattern]) => pattern.test(userAgent))?.[1]
  const system = systemPatterns.find(([pattern]) => pattern.test(userAgent))?.[1]
  if (browser && system) {
    return t('admin.sessions.deviceOn', { browser, os: system })
  }
  return browser ?? system ?? t('admin.sessions.unknownDevice')
}

async function revoke() {
  const target = revokeTarget.value
  if (!target) {
    return
  }
  try {
    await revokeMutation.mutateAsync(target.id)
    revokeTarget.value = null
  } catch {
    revokeTarget.value = null
  }
}
</script>

<template>
  <div class="space-y-4">
    <div class="space-y-1">
      <h2 class="text-lg font-semibold tracking-tight">
        {{ t('admin.sessions.title') }}
      </h2>
      <p class="text-sm text-muted-foreground">
        {{ t('admin.sessions.description') }}
      </p>
    </div>

    <Card>
      <CardContent class="pt-6">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-end">
          <div class="flex-1 space-y-2">
            <label
              for="admin-session-user"
              class="text-sm font-medium"
            >
              {{ t('admin.sessions.filters.user') }}
            </label>
            <Select
              :model-value="userId"
              @update:model-value="setUserId"
            >
              <SelectTrigger
                id="admin-session-user"
                class="w-full"
              >
                <SelectValue :placeholder="t('admin.sessions.filters.allUsers')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem
                  v-for="user in users"
                  :key="user.id"
                  :value="user.id"
                >
                  {{ user.email }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="w-full space-y-2 sm:w-56">
            <label
              for="admin-session-status"
              class="text-sm font-medium"
            >
              {{ t('admin.sessions.filters.status') }}
            </label>
            <Select
              :model-value="status === '' ? 'all' : status"
              @update:model-value="setStatus"
            >
              <SelectTrigger
                id="admin-session-status"
                class="w-full"
              >
                <SelectValue :placeholder="t('admin.sessions.filters.allStatuses')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">
                  {{ t('admin.sessions.filters.allStatuses') }}
                </SelectItem>
                <SelectItem value="active">
                  {{ t('admin.sessions.status.active') }}
                </SelectItem>
                <SelectItem value="revoked">
                  {{ t('admin.sessions.status.revoked') }}
                </SelectItem>
                <SelectItem value="expired">
                  {{ t('admin.sessions.status.expired') }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
      </CardContent>
    </Card>

    <AdminProblem :error="error" />

    <div
      v-if="isPending"
      class="space-y-2 rounded-xl border p-4"
      aria-busy="true"
    >
      <Skeleton class="h-5 w-1/3" />
      <Skeleton class="h-10 w-full" />
      <Skeleton class="h-10 w-full" />
      <Skeleton class="h-10 w-full" />
    </div>

    <p
      v-else-if="sessions.length === 0"
      class="rounded-lg border border-dashed px-4 py-8 text-center text-sm text-muted-foreground"
    >
      {{ t('admin.sessions.empty') }}
    </p>

    <template v-else>
      <Card>
        <Table class="min-w-[680px] table-fixed sm:min-w-full">
          <TableHeader>
            <TableRow>
              <TableHead class="w-[28%]">
                {{ t('admin.sessions.table.session') }}
              </TableHead>
              <TableHead class="w-[36%]">
                {{ t('admin.sessions.table.device') }}
              </TableHead>
              <TableHead class="w-[24%]">
                {{ t('admin.sessions.table.times') }}
              </TableHead>
              <TableHead class="w-[12%] text-right">
                {{ t('admin.common.actions') }}
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow
              v-for="session in sessions"
              :key="session.id"
            >
              <TableCell class="align-top whitespace-normal">
                <p class="font-medium break-all">
                  {{ session.userEmail }}
                </p>
                <div class="mt-1 flex flex-wrap items-center gap-2">
                  <Badge :variant="statusVariant(sessionStatus(session))">
                    {{ t(`admin.sessions.status.${sessionStatus(session)}`) }}
                  </Badge>
                  <span class="text-xs text-muted-foreground">
                    {{ session.ip ?? t('admin.sessions.unknownIp') }}
                  </span>
                </div>
              </TableCell>
              <TableCell class="align-top whitespace-normal">
                <p class="text-sm">
                  {{ deviceName(session.userAgent) }}
                </p>
                <p
                  v-if="session.userAgent"
                  class="mt-0.5 truncate text-xs text-muted-foreground"
                  :title="session.userAgent"
                >
                  {{ session.userAgent }}
                </p>
                <div class="mt-1.5 flex flex-wrap items-center gap-1">
                  <Badge
                    v-for="method in session.amr"
                    :key="method"
                    variant="outline"
                  >
                    {{ method }}
                  </Badge>
                  <span
                    v-if="session.amr.length === 0"
                    class="text-xs text-muted-foreground"
                  >
                    {{ t('admin.common.none') }}
                  </span>
                  <span
                    v-if="session.acr"
                    class="font-mono text-[11px] break-all text-muted-foreground"
                  >
                    {{ session.acr }}
                  </span>
                </div>
              </TableCell>
              <TableCell class="align-top whitespace-normal">
                <dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-0.5 text-xs">
                  <dt class="text-muted-foreground">
                    {{ t('admin.sessions.authTime') }}
                  </dt>
                  <dd>{{ formatCompactDate(session.authTime) }}</dd>
                  <dt class="text-muted-foreground">
                    {{ t('admin.sessions.lastSeen') }}
                  </dt>
                  <dd>{{ formatCompactDate(session.lastSeenAt) }}</dd>
                  <dt class="text-muted-foreground">
                    {{ t('admin.sessions.expires') }}
                  </dt>
                  <dd>{{ formatCompactDate(session.expiresAt) }}</dd>
                </dl>
              </TableCell>
              <TableCell class="align-top text-right">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  :disabled="session.revokedAt !== null"
                  @click="revokeTarget = session"
                >
                  {{ t('admin.sessions.revoke') }}
                </Button>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </Card>

      <div class="flex items-center justify-between gap-3">
        <p class="text-xs text-muted-foreground">
          {{ isFetching ? t('admin.common.refreshing') : t('admin.common.pageSize', { count: sessions.length }) }}
        </p>
        <AdminPager
          :next-after="data?.nextAfter ?? null"
          :has-previous="pager.hasPrevious.value"
          :pending="isFetching"
          @next="pager.next(data?.nextAfter ?? null)"
          @previous="pager.previous"
        />
      </div>
    </template>

    <AdminConfirmDialog
      :open="revokeTarget !== null"
      :title="t('admin.sessions.revokeTitle')"
      :description="
        t('admin.sessions.revokeDescription', {
          email: revokeTarget?.userEmail ?? '',
        })
      "
      :confirm-label="t('admin.sessions.revoke')"
      :pending-label="t('admin.sessions.revoking')"
      :pending="revokeMutation.isPending.value"
      destructive
      @update:open="revokeTarget = $event ? revokeTarget : null"
      @confirm="revoke"
    />
  </div>
</template>
