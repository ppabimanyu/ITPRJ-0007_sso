<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { useQuery } from '@tanstack/vue-query'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

import { getUser, listRoles } from '@/lib/admin/api'
import { adminKeys } from '@/lib/query-keys'

import AdminConfirmDialog from '../components/AdminConfirmDialog.vue'
import AdminProblem from '../components/AdminProblem.vue'
import {
  useAssignUserRoles,
  useResendUserInvite,
  useResetUserPassword,
  useUpdateUser,
} from '../queries'
import { useAdminError } from '../use-admin-error'
import { useAdminFormat } from '../use-admin-format'

const { t } = useI18n()
const route = useRoute()
const { formatDate } = useAdminFormat()
const { fieldErrors } = useAdminError()

const id = computed(() => {
  const value = route.params.id
  return typeof value === 'string' ? value : ''
})

const userQuery = useQuery({
  queryKey: computed(() => adminKeys.user(id.value)),
  queryFn: () => getUser(id.value),
  enabled: computed(() => id.value !== ''),
})

const rolesQuery = useQuery({
  queryKey: adminKeys.roles,
  queryFn: listRoles,
})

const profileUpdate = useUpdateUser()
const disableUpdate = useUpdateUser()
const rolesUpdate = useAssignUserRoles()
const resetMutation = useResetUserPassword()
const inviteMutation = useResendUserInvite()

const form = reactive({ name: '', locale: 'en' as 'en' | 'id' })

function setLocale(value: unknown) {
  form.locale = value === 'id' ? 'id' : 'en'
}

const selectedRoles = ref<string[]>([])

function toggleRole(roleId: string, checked: boolean | 'indeterminate') {
  if (checked) {
    if (!selectedRoles.value.includes(roleId)) {
      selectedRoles.value = [...selectedRoles.value, roleId]
    }
    return
  }
  selectedRoles.value = selectedRoles.value.filter((id) => id !== roleId)
}

const savedProfile = ref(false)
const savedRoles = ref(false)
const resetSent = ref(false)
const inviteSent = ref(false)
const confirmToggle = ref(false)

const user = computed(() => userQuery.data.value ?? null)
const status = computed(() => {
  if (!user.value) {
    return 'active'
  }
  if (user.value.disabledAt !== null) {
    return 'disabled'
  }
  return user.value.emailVerifiedAt === null ? 'pending' : 'active'
})

watch(
  () => user.value,
  (value) => {
    if (!value) {
      return
    }
    form.name = value.name ?? ''
    form.locale = value.locale === 'id' ? 'id' : 'en'
    selectedRoles.value = value.roles.map((role) => role.id)
  },
  { immediate: true },
)

const profileFields = computed(() => fieldErrors(profileUpdate.error.value))

function profileError(name: string): string {
  return profileFields.value[name] ?? ''
}

async function saveProfile() {
  savedProfile.value = false
  try {
    await profileUpdate.mutateAsync({
      id: id.value,
      values: { name: form.name.trim(), locale: form.locale },
    })
    savedProfile.value = true
  } catch {
    // The mutation error renders above the form.
  }
}

async function saveRoles() {
  savedRoles.value = false
  try {
    await rolesUpdate.mutateAsync({ id: id.value, roleIds: selectedRoles.value })
    savedRoles.value = true
  } catch {
    // The mutation error renders above the role list.
  }
}

async function toggleDisabled() {
  if (!user.value) {
    return
  }
  try {
    await disableUpdate.mutateAsync({
      id: id.value,
      values: { disabled: user.value.disabledAt === null },
    })
    confirmToggle.value = false
  } catch {
    confirmToggle.value = false
  }
}

async function resetPassword() {
  resetSent.value = false
  try {
    await resetMutation.mutateAsync(id.value)
    resetSent.value = true
  } catch {
    // The mutation error renders below the action buttons.
  }
}

async function resendInvite() {
  inviteSent.value = false
  try {
    await inviteMutation.mutateAsync(id.value)
    inviteSent.value = true
  } catch {
    // The mutation error renders below the action buttons.
  }
}
</script>

<template>
  <div class="space-y-6">
    <RouterLink
      to="/admin/users"
      class="text-sm font-medium text-primary underline-offset-4 hover:underline"
    >
      {{ t('admin.users.backToList') }}
    </RouterLink>

    <AdminProblem :error="userQuery.error.value" />

    <p
      v-if="userQuery.isPending.value"
      class="text-sm text-muted-foreground"
    >
      {{ t('common.loading') }}
    </p>

    <template v-else-if="user">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div class="space-y-1">
          <h2 class="text-xl font-semibold tracking-tight">
            {{ user.name ?? user.email }}
          </h2>
          <p class="text-sm text-muted-foreground">
            {{ user.email }}
          </p>
          <div class="flex flex-wrap items-center gap-2 pt-1">
            <Badge :variant="status === 'disabled' ? 'destructive' : status === 'pending' ? 'secondary' : 'outline'">
              {{ t(`admin.users.status.${status}`) }}
            </Badge>
            <Badge :variant="user.mfaEnrolled ? 'secondary' : 'outline'">
              {{ user.mfaEnrolled ? t('admin.users.detail.mfaOn') : t('admin.users.detail.mfaOff') }}
            </Badge>
          </div>
        </div>
      </div>

      <div class="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>{{ t('admin.users.detail.profileTitle') }}</CardTitle>
            <CardDescription>{{ t('admin.users.detail.profileDescription') }}</CardDescription>
          </CardHeader>
          <CardContent class="space-y-4">
            <Alert
              v-if="savedProfile"
            >
              <AlertDescription>{{ t('admin.users.detail.profileSaved') }}</AlertDescription>
            </Alert>
            <AdminProblem :error="profileUpdate.error.value" />

            <form
              class="space-y-4"
              novalidate
              @submit.prevent="saveProfile"
            >
              <div class="space-y-2">
                <Label for="admin-user-name">{{ t('admin.users.detail.name') }}</Label>
                <Input
                  id="admin-user-name"
                  v-model="form.name"
                  :aria-invalid="profileError('name') !== ''"
                />
                <p
                  v-if="profileError('name')"
                  role="alert"
                  class="text-sm text-destructive"
                >
                  {{ profileError('name') }}
                </p>
              </div>
              <div class="space-y-2">
                <Label for="admin-user-locale">{{ t('admin.users.detail.locale') }}</Label>
                <Select
                  :model-value="form.locale"
                  @update:model-value="setLocale"
                >
                  <SelectTrigger
                    id="admin-user-locale"
                    class="w-full"
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="en">
                      {{ t('admin.users.detail.localeEn') }}
                    </SelectItem>
                    <SelectItem value="id">
                      {{ t('admin.users.detail.localeId') }}
                    </SelectItem>
                  </SelectContent>
                </Select>
                <p
                  v-if="profileError('locale')"
                  role="alert"
                  class="text-sm text-destructive"
                >
                  {{ profileError('locale') }}
                </p>
              </div>
              <Button
                type="submit"
                :disabled="profileUpdate.isPending.value"
              >
                {{ profileUpdate.isPending.value ? t('admin.common.saving') : t('admin.common.save') }}
              </Button>
            </form>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{{ t('admin.users.detail.rolesTitle') }}</CardTitle>
            <CardDescription>{{ t('admin.users.detail.rolesDescription') }}</CardDescription>
          </CardHeader>
          <CardContent class="space-y-4">
            <Alert
              v-if="savedRoles"
            >
              <AlertDescription>{{ t('admin.users.detail.rolesSaved') }}</AlertDescription>
            </Alert>
            <AdminProblem :error="rolesUpdate.error.value" />

            <p
              v-if="rolesQuery.isPending.value"
              class="text-sm text-muted-foreground"
            >
              {{ t('common.loading') }}
            </p>
            <ul
              v-else
              class="space-y-2"
            >
              <li
                v-for="role in rolesQuery.data.value ?? []"
                :key="role.id"
                class="flex items-start gap-3 rounded-md border px-3 py-2"
              >
                <Checkbox
                  :id="`admin-role-${role.id}`"
                  :model-value="selectedRoles.includes(role.id)"
                  class="mt-1"
                  @update:model-value="(checked) => toggleRole(role.id, checked)"
                />
                <label
                  :for="`admin-role-${role.id}`"
                  class="flex-1 space-y-0.5"
                >
                  <span class="block text-sm font-medium">{{ role.name }}</span>
                  <span
                    v-if="role.description"
                    class="block text-xs text-muted-foreground"
                  >
                    {{ role.description }}
                  </span>
                </label>
                <Badge
                  v-if="role.isSystem"
                  variant="secondary"
                >
                  {{ t('admin.users.detail.systemRole') }}
                </Badge>
              </li>
            </ul>

            <Button
              type="button"
              :disabled="rolesUpdate.isPending.value"
              @click="saveRoles"
            >
              {{ rolesUpdate.isPending.value ? t('admin.common.saving') : t('admin.users.detail.saveRoles') }}
            </Button>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{{ t('admin.users.detail.accessTitle') }}</CardTitle>
          <CardDescription>{{ t('admin.users.detail.accessDescription') }}</CardDescription>
        </CardHeader>
        <CardContent class="space-y-4">
          <Alert
            v-if="resetSent"
          >
            <AlertDescription>{{ t('admin.users.detail.resetSent') }}</AlertDescription>
          </Alert>
          <Alert
            v-if="inviteSent"
          >
            <AlertDescription>{{ t('admin.users.detail.inviteSent') }}</AlertDescription>
          </Alert>
          <AdminProblem :error="disableUpdate.error.value" />
          <AdminProblem :error="resetMutation.error.value" />
          <AdminProblem :error="inviteMutation.error.value" />

          <div class="flex flex-wrap gap-2">
            <Button
              type="button"
              :variant="status === 'disabled' ? 'default' : 'destructive'"
              :disabled="disableUpdate.isPending.value"
              @click="confirmToggle = true"
            >
              {{
                status === 'disabled'
                  ? t('admin.users.detail.enable')
                  : t('admin.users.detail.disable')
              }}
            </Button>
            <Button
              type="button"
              variant="outline"
              :disabled="resetMutation.isPending.value || status === 'disabled'"
              @click="resetPassword"
            >
              {{
                resetMutation.isPending.value
                  ? t('admin.users.detail.resetting')
                  : t('admin.users.detail.resetPassword')
              }}
            </Button>
            <Button
              type="button"
              variant="outline"
              :disabled="inviteMutation.isPending.value || status === 'disabled'"
              @click="resendInvite"
            >
              {{
                inviteMutation.isPending.value
                  ? t('admin.users.detail.sendingInvite')
                  : t('admin.users.detail.resendInvite')
              }}
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{{ t('admin.users.detail.metadataTitle') }}</CardTitle>
        </CardHeader>
        <CardContent>
          <dl class="grid gap-x-6 gap-y-2 text-sm sm:grid-cols-[auto_1fr]">
            <dt class="text-muted-foreground">
              {{ t('admin.users.detail.id') }}
            </dt>
            <dd class="break-all font-mono text-xs">
              {{ user.id }}
            </dd>
            <dt class="text-muted-foreground">
              {{ t('admin.users.table.sessions') }}
            </dt>
            <dd>{{ user.sessionCount }}</dd>
            <dt class="text-muted-foreground">
              {{ t('admin.users.detail.created') }}
            </dt>
            <dd>{{ formatDate(user.createdAt) }}</dd>
            <dt class="text-muted-foreground">
              {{ t('admin.users.detail.updated') }}
            </dt>
            <dd>{{ formatDate(user.updatedAt) }}</dd>
          </dl>
        </CardContent>
      </Card>

      <AdminConfirmDialog
        :open="confirmToggle"
        :title="status === 'disabled' ? t('admin.users.detail.enableTitle') : t('admin.users.detail.disableTitle')"
        :description="
          status === 'disabled'
            ? t('admin.users.detail.enableDescription')
            : t('admin.users.detail.disableDescription')
        "
        :confirm-label="status === 'disabled' ? t('admin.users.detail.enable') : t('admin.users.detail.disable')"
        :pending-label="t('admin.common.saving')"
        :pending="disableUpdate.isPending.value"
        :destructive="status !== 'disabled'"
        @update:open="confirmToggle = $event"
        @confirm="toggleDisabled"
      />
    </template>
  </div>
</template>
