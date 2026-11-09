<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuery } from '@tanstack/vue-query'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

import { listRoles } from '@/lib/admin/api'
import { adminKeys } from '@/lib/query-keys'

import AdminProblem from '../components/AdminProblem.vue'
import CreateRoleDialog from './CreateRoleDialog.vue'

const { t } = useI18n()
const createOpen = ref(false)

const { data, isPending, error } = useQuery({
  queryKey: adminKeys.roles,
  queryFn: listRoles,
})
</script>

<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="space-y-1">
        <h2 class="text-lg font-semibold tracking-tight">
          {{ t('admin.roles.title') }}
        </h2>
        <p class="text-sm text-muted-foreground">
          {{ t('admin.roles.description') }}
        </p>
      </div>
      <Button @click="createOpen = true">
        {{ t('admin.roles.create.action') }}
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
      {{ t('admin.roles.empty') }}
    </p>

    <Card v-else>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{{ t('admin.roles.table.name') }}</TableHead>
            <TableHead>{{ t('admin.roles.table.description') }}</TableHead>
            <TableHead>{{ t('admin.roles.table.kind') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow
            v-for="role in data ?? []"
            :key="role.id"
          >
            <TableCell class="font-medium">
              {{ role.name }}
            </TableCell>
            <TableCell class="text-muted-foreground">
              {{ role.description ?? t('admin.common.none') }}
            </TableCell>
            <TableCell>
              <Badge :variant="role.isSystem ? 'secondary' : 'outline'">
                {{ role.isSystem ? t('admin.roles.table.system') : t('admin.roles.table.custom') }}
              </Badge>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </Card>

    <CreateRoleDialog v-model:open="createOpen" />
  </div>
</template>
