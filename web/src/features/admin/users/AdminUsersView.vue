<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuery } from '@tanstack/vue-query'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

import { listUsers } from '@/lib/admin/api'
import { adminKeys } from '@/lib/query-keys'

import AdminPager from '../components/AdminPager.vue'
import AdminProblem from '../components/AdminProblem.vue'
import { useAdminFormat } from '../use-admin-format'
import { useAdminPager } from '../use-admin-pager'
import CreateUserDialog from './CreateUserDialog.vue'

type UserStatus = '' | 'active' | 'pending' | 'disabled'

const { t } = useI18n()
const { formatDate } = useAdminFormat()
const pager = useAdminPager()

const search = ref('')
const appliedSearch = ref('')
const status = ref<UserStatus>('')
const createOpen = ref(false)

const params = computed(() => ({
  q: appliedSearch.value === '' ? undefined : appliedSearch.value,
  status: status.value === '' ? undefined : status.value,
  after: pager.after.value ?? undefined,
  limit: 25,
}))

const { data, isPending, isFetching, error } = useQuery({
  queryKey: computed(() => adminKeys.userList(params.value)),
  queryFn: () => listUsers(params.value),
})

const users = computed(() => data.value?.items ?? [])

watch(status, () => {
  pager.reset()
})

function setStatus(value: unknown) {
  status.value = value === 'all' ? '' : (value as UserStatus)
}

function applySearch() {
  appliedSearch.value = search.value.trim()
  pager.reset()
}

function userStatus(user: (typeof users.value)[number]): 'active' | 'pending' | 'disabled' {
  if (user.disabledAt !== null) {
    return 'disabled'
  }
  if (user.emailVerifiedAt === null) {
    return 'pending'
  }
  return 'active'
}

function statusVariant(value: 'active' | 'pending' | 'disabled'): 'destructive' | 'secondary' | 'outline' {
  if (value === 'disabled') {
    return 'destructive'
  }
  return value === 'pending' ? 'secondary' : 'outline'
}
</script>

<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="space-y-1">
        <h2 class="text-lg font-semibold tracking-tight">
          {{ t('admin.users.title') }}
        </h2>
        <p class="text-sm text-muted-foreground">
          {{ t('admin.users.description') }}
        </p>
      </div>
      <Button @click="createOpen = true">
        {{ t('admin.users.create.action') }}
      </Button>
    </div>

    <Card>
      <CardContent class="pt-6">
        <form
          class="flex flex-col gap-3 sm:flex-row sm:items-end"
          @submit.prevent="applySearch"
        >
          <div class="flex-1 space-y-2">
            <label
              for="admin-user-search"
              class="text-sm font-medium"
            >
              {{ t('admin.users.filters.search') }}
            </label>
            <Input
              id="admin-user-search"
              v-model="search"
              :placeholder="t('admin.users.filters.searchPlaceholder')"
            />
          </div>
          <div class="w-full space-y-2 sm:w-48">
            <label
              for="admin-user-status"
              class="text-sm font-medium"
            >
              {{ t('admin.users.filters.status') }}
            </label>
            <Select
              :model-value="status === '' ? 'all' : status"
              @update:model-value="setStatus"
            >
              <SelectTrigger
                id="admin-user-status"
                class="w-full"
              >
                <SelectValue :placeholder="t('admin.users.filters.allStatuses')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">
                  {{ t('admin.users.filters.allStatuses') }}
                </SelectItem>
                <SelectItem value="active">
                  {{ t('admin.users.status.active') }}
                </SelectItem>
                <SelectItem value="pending">
                  {{ t('admin.users.status.pending') }}
                </SelectItem>
                <SelectItem value="disabled">
                  {{ t('admin.users.status.disabled') }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
          <Button
            type="submit"
            variant="outline"
          >
            {{ t('admin.common.search') }}
          </Button>
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
      v-else-if="users.length === 0"
      class="rounded-lg border border-dashed px-4 py-8 text-center text-sm text-muted-foreground"
    >
      {{ t('admin.users.empty') }}
    </p>

    <template v-else>
      <Card>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{{ t('admin.users.table.user') }}</TableHead>
              <TableHead>{{ t('admin.users.table.status') }}</TableHead>
              <TableHead>{{ t('admin.users.table.roles') }}</TableHead>
              <TableHead>{{ t('admin.users.table.mfa') }}</TableHead>
              <TableHead>{{ t('admin.users.table.sessions') }}</TableHead>
              <TableHead>{{ t('admin.users.table.created') }}</TableHead>
              <TableHead class="text-right">
                {{ t('admin.common.actions') }}
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow
              v-for="user in users"
              :key="user.id"
            >
              <TableCell>
                <div class="space-y-0.5">
                  <p class="font-medium">
                    {{ user.name ?? user.email }}
                  </p>
                  <p class="text-xs text-muted-foreground">
                    {{ user.email }}
                  </p>
                </div>
              </TableCell>
              <TableCell>
                <Badge :variant="statusVariant(userStatus(user))">
                  {{ t(`admin.users.status.${userStatus(user)}`) }}
                </Badge>
              </TableCell>
              <TableCell>
                <div
                  v-if="user.roles.length > 0"
                  class="flex flex-wrap gap-1"
                >
                  <Badge
                    v-for="role in user.roles"
                    :key="role.id"
                    variant="outline"
                  >
                    {{ role.name }}
                  </Badge>
                </div>
                <span
                  v-else
                  class="text-xs text-muted-foreground"
                >
                  {{ t('admin.common.none') }}
                </span>
              </TableCell>
              <TableCell>
                <Badge :variant="user.mfaEnrolled ? 'secondary' : 'outline'">
                  {{ user.mfaEnrolled ? t('admin.common.on') : t('admin.common.off') }}
                </Badge>
              </TableCell>
              <TableCell>{{ user.sessionCount }}</TableCell>
              <TableCell class="whitespace-nowrap text-xs text-muted-foreground">
                {{ formatDate(user.createdAt) }}
              </TableCell>
              <TableCell class="text-right">
                <Button
                  as-child
                  variant="outline"
                  size="sm"
                >
                  <RouterLink :to="{ name: 'admin-user', params: { id: user.id } }">
                    {{ t('admin.common.open') }}
                  </RouterLink>
                </Button>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </Card>

      <div class="flex items-center justify-between gap-3">
        <p class="text-xs text-muted-foreground">
          {{ isFetching ? t('admin.common.refreshing') : t('admin.common.pageSize', { count: users.length }) }}
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

    <CreateUserDialog
      v-model:open="createOpen"
      @created="pager.reset"
    />
  </div>
</template>
