<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuery } from '@tanstack/vue-query'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

import { listClients } from '@/lib/admin/api'
import type { AdminClientWithSecret } from '@/lib/admin/types'
import { adminKeys } from '@/lib/query-keys'

import AdminPager from '../components/AdminPager.vue'
import AdminProblem from '../components/AdminProblem.vue'
import AdminSecretDialog from '../components/AdminSecretDialog.vue'
import { useAdminFormat } from '../use-admin-format'
import { useAdminPager } from '../use-admin-pager'
import CreateClientDialog from './CreateClientDialog.vue'

const { t } = useI18n()
const { formatDate } = useAdminFormat()
const pager = useAdminPager()

const createOpen = ref(false)
const secretOpen = ref(false)
const secretValue = ref('')
const secretClientName = ref('')
const createdNotice = ref('')
const copiedClientId = ref('')
let copyTimer: ReturnType<typeof setTimeout> | undefined

const params = computed(() => ({
  after: pager.after.value ?? undefined,
  limit: 25,
}))

const { data, isPending, isFetching, error } = useQuery({
  queryKey: computed(() => adminKeys.clientList(params.value)),
  queryFn: () => listClients(params.value),
})

const clients = computed(() => data.value?.items ?? [])

function onCreated(client: AdminClientWithSecret) {
  createdNotice.value = ''
  if (client.clientSecret !== null) {
    secretValue.value = client.clientSecret
    secretClientName.value = client.name
    secretOpen.value = true
    return
  }
  createdNotice.value = client.name
  pager.reset()
}

async function copyClientId(clientId: string) {
  try {
    await navigator.clipboard.writeText(clientId)
    copiedClientId.value = clientId
    if (copyTimer !== undefined) {
      clearTimeout(copyTimer)
    }
    copyTimer = setTimeout(() => {
      copiedClientId.value = ''
    }, 2000)
  } catch {
    copiedClientId.value = ''
  }
}

onBeforeUnmount(() => {
  if (copyTimer !== undefined) {
    clearTimeout(copyTimer)
  }
})
</script>

<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="space-y-1">
        <h2 class="text-lg font-semibold tracking-tight">
          {{ t('admin.clients.title') }}
        </h2>
        <p class="text-sm text-muted-foreground">
          {{ t('admin.clients.description') }}
        </p>
      </div>
      <Button @click="createOpen = true">
        {{ t('admin.clients.create.action') }}
      </Button>
    </div>

    <Alert
      v-if="createdNotice"
    >
      <AlertDescription>
        {{ t('admin.clients.create.publicCreated', { name: createdNotice }) }}
      </AlertDescription>
    </Alert>

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
      v-else-if="clients.length === 0"
      class="rounded-lg border border-dashed px-4 py-8 text-center text-sm text-muted-foreground"
    >
      {{ t('admin.clients.empty') }}
    </p>

    <template v-else>
      <Card>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{{ t('admin.clients.table.client') }}</TableHead>
              <TableHead>{{ t('admin.clients.table.type') }}</TableHead>
              <TableHead>{{ t('admin.clients.table.status') }}</TableHead>
              <TableHead>{{ t('admin.clients.table.redirects') }}</TableHead>
              <TableHead>{{ t('admin.clients.table.created') }}</TableHead>
              <TableHead class="text-right">
                {{ t('admin.common.actions') }}
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow
              v-for="client in clients"
              :key="client.id"
            >
              <TableCell>
                <div class="space-y-0.5">
                  <p class="font-medium">
                    {{ client.name }}
                  </p>
                  <p class="font-mono text-xs text-muted-foreground">
                    {{ client.clientId }}
                  </p>
                </div>
              </TableCell>
              <TableCell>
                <Badge variant="outline">
                  {{ t(`admin.clients.type.${client.type}`) }}
                </Badge>
              </TableCell>
              <TableCell>
                <Badge :variant="client.disabled ? 'destructive' : 'secondary'">
                  {{ client.disabled ? t('admin.clients.status.disabled') : t('admin.clients.status.active') }}
                </Badge>
              </TableCell>
              <TableCell>{{ client.redirectUris.length }}</TableCell>
              <TableCell class="whitespace-nowrap text-xs text-muted-foreground">
                {{ formatDate(client.createdAt) }}
              </TableCell>
              <TableCell class="text-right">
                <DropdownMenu>
                  <DropdownMenuTrigger as-child>
                    <Button
                      variant="outline"
                      size="sm"
                    >
                      {{ t('admin.common.actions') }}
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem as-child>
                      <RouterLink :to="{ name: 'admin-client', params: { id: client.id } }">
                        {{ t('admin.common.open') }}
                      </RouterLink>
                    </DropdownMenuItem>
                    <DropdownMenuItem @select="copyClientId(client.clientId)">
                      {{ copiedClientId === client.clientId ? t('admin.common.copied') : t('admin.clients.table.copyId') }}
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </Card>

      <div class="flex items-center justify-between gap-3">
        <p class="text-xs text-muted-foreground">
          {{ isFetching ? t('admin.common.refreshing') : t('admin.common.pageSize', { count: clients.length }) }}
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

    <CreateClientDialog
      v-model:open="createOpen"
      @created="onCreated"
    />

    <AdminSecretDialog
      v-model:open="secretOpen"
      :title="t('admin.clients.secret.createdTitle', { name: secretClientName })"
      :description="t('admin.clients.secret.createdDescription')"
      :secret="secretValue"
      :warning="t('admin.clients.secret.warning')"
    />
  </div>
</template>
