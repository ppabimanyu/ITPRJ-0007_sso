<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuery } from '@tanstack/vue-query'

import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

import { listGroups } from '@/lib/admin/api'
import { adminKeys } from '@/lib/query-keys'

import AdminProblem from '../components/AdminProblem.vue'
import CreateGroupDialog from './CreateGroupDialog.vue'

const { t } = useI18n()
const createOpen = ref(false)

const { data, isPending, error } = useQuery({
  queryKey: adminKeys.groups,
  queryFn: listGroups,
})
</script>

<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="space-y-1">
        <h2 class="text-lg font-semibold tracking-tight">
          {{ t('admin.groups.title') }}
        </h2>
        <p class="text-sm text-muted-foreground">
          {{ t('admin.groups.description') }}
        </p>
      </div>
      <Button @click="createOpen = true">
        {{ t('admin.groups.create.action') }}
      </Button>
    </div>

    <AdminProblem :error="error" />

    <p
      v-if="isPending"
      class="text-sm text-muted-foreground"
    >
      {{ t('common.loading') }}
    </p>

    <p
      v-else-if="(data ?? []).length === 0"
      class="rounded-lg border border-dashed px-4 py-8 text-center text-sm text-muted-foreground"
    >
      {{ t('admin.groups.empty') }}
    </p>

    <Card v-else>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{{ t('admin.groups.table.name') }}</TableHead>
            <TableHead>{{ t('admin.groups.table.description') }}</TableHead>
            <TableHead>{{ t('admin.groups.table.members') }}</TableHead>
            <TableHead class="text-right">
              {{ t('admin.common.actions') }}
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow
            v-for="group in data ?? []"
            :key="group.id"
          >
            <TableCell class="font-medium">
              {{ group.name }}
            </TableCell>
            <TableCell class="text-muted-foreground">
              {{ group.description ?? t('admin.common.none') }}
            </TableCell>
            <TableCell>{{ group.memberCount }}</TableCell>
            <TableCell class="text-right">
              <Button
                as-child
                variant="outline"
                size="sm"
              >
                <RouterLink :to="{ name: 'admin-group', params: { id: group.id } }">
                  {{ t('admin.common.open') }}
                </RouterLink>
              </Button>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </Card>

    <CreateGroupDialog v-model:open="createOpen" />
  </div>
</template>
