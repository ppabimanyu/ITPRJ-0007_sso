<script setup lang="ts">
import { computed, reactive } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuery } from '@tanstack/vue-query'

import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

import { listAuditEvents } from '@/lib/admin/api'
import { adminKeys } from '@/lib/query-keys'

import AdminPager from '../components/AdminPager.vue'
import AdminProblem from '../components/AdminProblem.vue'
import { useAdminFormat } from '../use-admin-format'
import { useAdminPager } from '../use-admin-pager'

const { t } = useI18n()
const { formatDate } = useAdminFormat()
const pager = useAdminPager()

const filters = reactive({
  from: '',
  to: '',
  type: '',
  actorId: '',
})

const applied = reactive({
  from: '',
  to: '',
  type: '',
  actorId: '',
})

const params = computed(() => ({
  from: toRfc3339(applied.from),
  to: toRfc3339(applied.to),
  type: applied.type.trim() === '' ? undefined : applied.type.trim(),
  actorId: applied.actorId.trim() === '' ? undefined : applied.actorId.trim(),
  after: pager.after.value ?? undefined,
  limit: 25,
}))

const { data, isPending, isFetching, error } = useQuery({
  queryKey: computed(() => adminKeys.auditList(params.value)),
  queryFn: () => listAuditEvents(params.value),
})

const events = computed(() => data.value?.items ?? [])

function toRfc3339(value: string): string | undefined {
  if (value === '') {
    return undefined
  }
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? undefined : date.toISOString()
}

function applyFilters() {
  applied.from = filters.from
  applied.to = filters.to
  applied.type = filters.type
  applied.actorId = filters.actorId
  pager.reset()
}

function metadataText(value: unknown): string {
  const text = JSON.stringify(value, null, 2)
  return text === undefined ? String(value) : text
}
</script>

<template>
  <div class="space-y-4">
    <div class="space-y-1">
      <h2 class="text-lg font-semibold tracking-tight">
        {{ t('admin.audit.title') }}
      </h2>
      <p class="text-sm text-muted-foreground">
        {{ t('admin.audit.description') }}
      </p>
    </div>

    <Card>
      <CardContent class="pt-6">
        <form
          class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4"
          @submit.prevent="applyFilters"
        >
          <div class="space-y-2">
            <label
              for="audit-from"
              class="text-sm font-medium"
            >
              {{ t('admin.audit.filters.from') }}
            </label>
            <Input
              id="audit-from"
              v-model="filters.from"
              type="datetime-local"
            />
          </div>
          <div class="space-y-2">
            <label
              for="audit-to"
              class="text-sm font-medium"
            >
              {{ t('admin.audit.filters.to') }}
            </label>
            <Input
              id="audit-to"
              v-model="filters.to"
              type="datetime-local"
            />
          </div>
          <div class="space-y-2">
            <label
              for="audit-type"
              class="text-sm font-medium"
            >
              {{ t('admin.audit.filters.type') }}
            </label>
            <Input
              id="audit-type"
              v-model="filters.type"
              :placeholder="t('admin.audit.filters.typePlaceholder')"
            />
          </div>
          <div class="space-y-2">
            <label
              for="audit-actor"
              class="text-sm font-medium"
            >
              {{ t('admin.audit.filters.actor') }}
            </label>
            <Input
              id="audit-actor"
              v-model="filters.actorId"
              :placeholder="t('admin.audit.filters.actorPlaceholder')"
            />
          </div>
          <div class="sm:col-span-2 lg:col-span-4">
            <Button
              type="submit"
              variant="outline"
            >
              {{ t('admin.audit.filters.apply') }}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>

    <AdminProblem :error="error" />

    <p
      v-if="isPending"
      class="text-sm text-muted-foreground"
    >
      {{ t('common.loading') }}
    </p>

    <p
      v-else-if="events.length === 0"
      class="rounded-lg border border-dashed px-4 py-8 text-center text-sm text-muted-foreground"
    >
      {{ t('admin.audit.empty') }}
    </p>

    <template v-else>
      <Card>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{{ t('admin.audit.table.time') }}</TableHead>
              <TableHead>{{ t('admin.audit.table.type') }}</TableHead>
              <TableHead>{{ t('admin.audit.table.actor') }}</TableHead>
              <TableHead>{{ t('admin.audit.table.metadata') }}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow
              v-for="event in events"
              :key="event.id"
            >
              <TableCell class="whitespace-nowrap text-xs text-muted-foreground">
                {{ formatDate(event.createdAt) }}
              </TableCell>
              <TableCell class="font-mono text-xs">
                {{ event.type }}
              </TableCell>
              <TableCell>
                <div class="space-y-0.5 text-xs">
                  <p class="font-medium">
                    {{ event.actorType }}
                  </p>
                  <p class="break-all text-muted-foreground">
                    {{ event.actorId ?? t('admin.common.none') }}
                  </p>
                </div>
              </TableCell>
              <TableCell>
                <details class="max-w-md">
                  <summary class="cursor-pointer text-xs font-medium text-primary">
                    {{ t('admin.audit.showMetadata') }}
                  </summary>
                  <pre class="mt-2 max-h-60 overflow-auto whitespace-pre-wrap break-all rounded-md border bg-muted/40 p-2 font-mono text-xs">{{ metadataText(event.metadata) }}</pre>
                </details>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </Card>

      <div class="flex items-center justify-between gap-3">
        <p class="text-xs text-muted-foreground">
          {{ isFetching ? t('admin.common.refreshing') : t('admin.common.pageSize', { count: events.length }) }}
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
  </div>
</template>
