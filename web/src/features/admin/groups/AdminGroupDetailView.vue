<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useQuery } from '@tanstack/vue-query'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'

import { getGroup, listGroupMembers, listUsers } from '@/lib/admin/api'
import { adminKeys } from '@/lib/query-keys'

import AdminConfirmDialog from '../components/AdminConfirmDialog.vue'
import AdminProblem from '../components/AdminProblem.vue'
import {
  useAddGroupMembers,
  useDeleteGroup,
  useRemoveGroupMember,
  useUpdateGroup,
} from '../queries'
import { useAdminError } from '../use-admin-error'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const { fieldErrors } = useAdminError()

const id = computed(() => {
  const value = route.params.id
  return typeof value === 'string' ? value : ''
})

const groupQuery = useQuery({
  queryKey: computed(() => adminKeys.group(id.value)),
  queryFn: () => getGroup(id.value),
  enabled: computed(() => id.value !== ''),
})

const membersQuery = useQuery({
  queryKey: computed(() => adminKeys.groupMembers(id.value)),
  queryFn: () => listGroupMembers(id.value),
  enabled: computed(() => id.value !== ''),
})

const usersQuery = useQuery({
  queryKey: computed(() => adminKeys.userList({ limit: 100 })),
  queryFn: () => listUsers({ limit: 100 }),
})

const updateMutation = useUpdateGroup()
const deleteMutation = useDeleteGroup()
const addMutation = useAddGroupMembers()
const removeMutation = useRemoveGroupMember()

const form = reactive({ name: '', description: '' })
const saved = ref(false)
const confirmDelete = ref(false)
const selectedUserId = ref('')

function setSelectedUser(value: unknown) {
  selectedUserId.value = String(value)
}

const group = computed(() => groupQuery.data.value ?? null)
const members = computed(() => membersQuery.data.value ?? [])
const updateFields = computed(() => fieldErrors(updateMutation.error.value))

watch(
  () => group.value,
  (value) => {
    if (!value) {
      return
    }
    form.name = value.name
    form.description = value.description ?? ''
  },
  { immediate: true },
)

const availableUsers = computed(() => {
  const memberIds = new Set(members.value.map((member) => member.userId))
  return (usersQuery.data.value?.items ?? []).filter((user) => !memberIds.has(user.id))
})

const removingUserId = computed(() => {
  if (!removeMutation.isPending.value) {
    return ''
  }
  return removeMutation.variables.value?.userId ?? ''
})

async function save() {
  saved.value = false
  try {
    await updateMutation.mutateAsync({
      id: id.value,
      values: {
        name: form.name.trim(),
        description: form.description.trim() === '' ? null : form.description.trim(),
      },
    })
    saved.value = true
  } catch {
    // The mutation error renders above the form.
  }
}

async function addMember() {
  if (selectedUserId.value === '') {
    return
  }
  try {
    await addMutation.mutateAsync({ id: id.value, userIds: [selectedUserId.value] })
    selectedUserId.value = ''
  } catch {
    // The mutation error renders above the member list.
  }
}

async function removeMember(userId: string) {
  try {
    await removeMutation.mutateAsync({ id: id.value, userId })
  } catch {
    // The mutation error renders above the member list.
  }
}

async function removeGroup() {
  try {
    await deleteMutation.mutateAsync(id.value)
    await router.push({ name: 'admin-groups' })
  } catch {
    confirmDelete.value = false
  }
}
</script>

<template>
  <div class="space-y-6">
    <RouterLink
      to="/admin/groups"
      class="text-sm font-medium text-primary underline-offset-4 hover:underline"
    >
      {{ t('admin.groups.backToList') }}
    </RouterLink>

    <AdminProblem :error="groupQuery.error.value" />

    <p
      v-if="groupQuery.isPending.value"
      class="text-sm text-muted-foreground"
    >
      {{ t('common.loading') }}
    </p>

    <template v-else-if="group">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div class="space-y-1">
          <h2 class="text-xl font-semibold tracking-tight">
            {{ group.name }}
          </h2>
          <p class="text-sm text-muted-foreground">
            {{ t('admin.groups.detail.memberCount', { count: group.memberCount }) }}
          </p>
        </div>
        <Button
          type="button"
          variant="destructive"
          :disabled="deleteMutation.isPending.value"
          @click="confirmDelete = true"
        >
          {{ deleteMutation.isPending.value ? t('admin.groups.detail.deleting') : t('admin.groups.detail.delete') }}
        </Button>
      </div>

      <Alert
        v-if="saved"
        variant="success"
      >
        <AlertDescription>{{ t('admin.groups.detail.saved') }}</AlertDescription>
      </Alert>
      <AdminProblem :error="updateMutation.error.value" />
      <AdminProblem :error="deleteMutation.error.value" />

      <Card>
        <CardHeader>
          <CardTitle>{{ t('admin.groups.detail.formTitle') }}</CardTitle>
          <CardDescription>{{ t('admin.groups.detail.formDescription') }}</CardDescription>
        </CardHeader>
        <CardContent>
          <form
            class="space-y-4"
            novalidate
            @submit.prevent="save"
          >
            <div class="space-y-2">
              <Label for="group-name">{{ t('admin.groups.detail.name') }}</Label>
              <Input
                id="group-name"
                v-model="form.name"
                :aria-invalid="updateFields.name !== undefined"
              />
              <p
                v-if="updateFields.name"
                role="alert"
                class="text-sm text-destructive"
              >
                {{ updateFields.name }}
              </p>
            </div>
            <div class="space-y-2">
              <Label for="group-description">{{ t('admin.groups.detail.description') }}</Label>
              <Textarea
                id="group-description"
                v-model="form.description"
                :aria-invalid="updateFields.description !== undefined"
              />
              <p
                v-if="updateFields.description"
                role="alert"
                class="text-sm text-destructive"
              >
                {{ updateFields.description }}
              </p>
            </div>
            <Button
              type="submit"
              :disabled="updateMutation.isPending.value"
            >
              {{ updateMutation.isPending.value ? t('admin.common.saving') : t('admin.common.save') }}
            </Button>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{{ t('admin.groups.detail.membersTitle') }}</CardTitle>
          <CardDescription>{{ t('admin.groups.detail.membersDescription') }}</CardDescription>
        </CardHeader>
        <CardContent class="space-y-4">
          <AdminProblem :error="addMutation.error.value" />
          <AdminProblem :error="removeMutation.error.value" />

          <div class="flex flex-col gap-2 sm:flex-row sm:items-end">
            <div class="flex-1 space-y-2">
              <Label for="group-add-member">{{ t('admin.groups.detail.addMember') }}</Label>
              <Select
                :model-value="selectedUserId"
                @update:model-value="setSelectedUser"
              >
                <SelectTrigger
                  id="group-add-member"
                  class="w-full"
                >
                  <SelectValue :placeholder="t('admin.groups.detail.selectUser')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem
                    v-for="user in availableUsers"
                    :key="user.id"
                    :value="user.id"
                  >
                    {{ user.email }}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
            <Button
              type="button"
              :disabled="addMutation.isPending.value || selectedUserId === ''"
              @click="addMember"
            >
              {{ addMutation.isPending.value ? t('admin.groups.detail.adding') : t('admin.groups.detail.add') }}
            </Button>
          </div>
          <p
            v-if="availableUsers.length === 0"
            class="text-xs text-muted-foreground"
          >
            {{ t('admin.groups.detail.noAvailableUsers') }}
          </p>

          <p
            v-if="membersQuery.isPending.value"
            class="text-sm text-muted-foreground"
          >
            {{ t('common.loading') }}
          </p>

          <p
            v-else-if="members.length === 0"
            class="rounded-lg border border-dashed px-4 py-6 text-center text-sm text-muted-foreground"
          >
            {{ t('admin.groups.detail.emptyMembers') }}
          </p>

          <ul
            v-else
            class="divide-y rounded-lg border"
          >
            <li
              v-for="member in members"
              :key="member.userId"
              class="flex flex-col gap-2 px-4 py-3 sm:flex-row sm:items-center sm:justify-between"
            >
              <div class="space-y-0.5">
                <p class="text-sm font-medium">
                  {{ member.name ?? member.email }}
                </p>
                <p class="text-xs text-muted-foreground">
                  {{ member.email }}
                </p>
              </div>
              <Button
                type="button"
                variant="outline"
                size="sm"
                :disabled="removeMutation.isPending.value && removingUserId === member.userId"
                @click="removeMember(member.userId)"
              >
                {{
                  removeMutation.isPending.value && removingUserId === member.userId
                    ? t('admin.groups.detail.removing')
                    : t('admin.groups.detail.remove')
                }}
              </Button>
            </li>
          </ul>
        </CardContent>
      </Card>

      <AdminConfirmDialog
        :open="confirmDelete"
        :title="t('admin.groups.detail.deleteTitle')"
        :description="t('admin.groups.detail.deleteDescription')"
        :confirm-label="t('admin.groups.detail.delete')"
        :pending-label="t('admin.groups.detail.deleting')"
        :pending="deleteMutation.isPending.value"
        destructive
        @update:open="confirmDelete = $event"
        @confirm="removeGroup"
      />
    </template>
  </div>
</template>
