<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useQuery } from '@tanstack/vue-query'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

import { getClient } from '@/lib/admin/api'
import { adminKeys } from '@/lib/query-keys'

import AdminConfirmDialog from '../components/AdminConfirmDialog.vue'
import AdminProblem from '../components/AdminProblem.vue'
import AdminSecretDialog from '../components/AdminSecretDialog.vue'
import { useDeleteClient, useRotateClientSecret, useUpdateClient } from '../queries'
import { useAdminError } from '../use-admin-error'
import { useAdminFormat } from '../use-admin-format'
import ClientForm from './ClientForm.vue'
import type { ClientFormValues } from './client-form'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const { formatDate } = useAdminFormat()
const { fieldErrors } = useAdminError()

const id = computed(() => {
  const value = route.params.id
  return typeof value === 'string' ? value : ''
})

const clientQuery = useQuery({
  queryKey: computed(() => adminKeys.client(id.value)),
  queryFn: () => getClient(id.value),
  enabled: computed(() => id.value !== ''),
})

const updateMutation = useUpdateClient()
const deleteMutation = useDeleteClient()
const rotateMutation = useRotateClientSecret()

const confirmDelete = ref(false)
const confirmRotate = ref(false)
const secretOpen = ref(false)
const secretValue = ref('')
const saved = ref(false)

const client = computed(() => clientQuery.data.value ?? null)
const fields = computed(() => fieldErrors(updateMutation.error.value))

async function save(values: ClientFormValues) {
  saved.value = false
  try {
    await updateMutation.mutateAsync({ id: id.value, values })
    saved.value = true
  } catch {
    // The mutation error renders above the form.
  }
}

async function remove() {
  try {
    await deleteMutation.mutateAsync(id.value)
    await router.push({ name: 'admin-clients' })
  } catch {
    confirmDelete.value = false
  }
}

async function rotate() {
  try {
    const result = await rotateMutation.mutateAsync(id.value)
    secretValue.value = result.clientSecret
    secretOpen.value = true
    confirmRotate.value = false
  } catch {
    confirmRotate.value = false
  }
}
</script>

<template>
  <div class="space-y-6">
    <RouterLink
      to="/admin/clients"
      class="text-sm font-medium text-primary underline-offset-4 hover:underline"
    >
      {{ t('admin.clients.backToList') }}
    </RouterLink>

    <AdminProblem :error="clientQuery.error.value" />

    <p
      v-if="clientQuery.isPending.value"
      class="text-sm text-muted-foreground"
    >
      {{ t('common.loading') }}
    </p>

    <template v-else-if="client">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div class="space-y-1">
          <h2 class="text-xl font-semibold tracking-tight">
            {{ client.name }}
          </h2>
          <p class="font-mono text-sm text-muted-foreground">
            {{ client.clientId }}
          </p>
          <div class="flex flex-wrap items-center gap-2 pt-1">
            <Badge variant="outline">
              {{ t(`admin.clients.type.${client.type}`) }}
            </Badge>
            <Badge :variant="client.disabled ? 'destructive' : 'secondary'">
              {{ client.disabled ? t('admin.clients.status.disabled') : t('admin.clients.status.active') }}
            </Badge>
          </div>
        </div>
        <div class="flex flex-wrap gap-2">
          <Button
            v-if="client.type === 'confidential'"
            type="button"
            variant="outline"
            :disabled="rotateMutation.isPending.value"
            @click="confirmRotate = true"
          >
            {{
              rotateMutation.isPending.value
                ? t('admin.clients.detail.rotating')
                : t('admin.clients.detail.rotateSecret')
            }}
          </Button>
          <Button
            type="button"
            variant="destructive"
            :disabled="deleteMutation.isPending.value"
            @click="confirmDelete = true"
          >
            {{
              deleteMutation.isPending.value
                ? t('admin.clients.detail.deleting')
                : t('admin.clients.detail.delete')
            }}
          </Button>
        </div>
      </div>

      <Alert
        v-if="saved"
      >
        <AlertDescription>{{ t('admin.clients.detail.saved') }}</AlertDescription>
      </Alert>
      <AdminProblem :error="updateMutation.error.value" />
      <AdminProblem :error="deleteMutation.error.value" />
      <AdminProblem :error="rotateMutation.error.value" />

      <Card>
        <CardHeader>
          <CardTitle>{{ t('admin.clients.detail.formTitle') }}</CardTitle>
          <CardDescription>{{ t('admin.clients.detail.formDescription') }}</CardDescription>
        </CardHeader>
        <CardContent>
          <ClientForm
            :initial="client"
            :pending="updateMutation.isPending.value"
            :field-errors="fields"
            :submit-label="t('admin.common.save')"
            :pending-label="t('admin.common.saving')"
            :allow-type-change="false"
            :show-disabled="true"
            @submit="save"
            @cancel="router.push({ name: 'admin-clients' })"
          />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{{ t('admin.clients.detail.metadataTitle') }}</CardTitle>
        </CardHeader>
        <CardContent>
          <dl class="grid gap-x-6 gap-y-2 text-sm sm:grid-cols-[auto_1fr]">
            <dt class="text-muted-foreground">
              {{ t('admin.clients.detail.id') }}
            </dt>
            <dd class="break-all font-mono text-xs">
              {{ client.id }}
            </dd>
            <dt class="text-muted-foreground">
              {{ t('admin.clients.detail.secretRotatedAt') }}
            </dt>
            <dd>{{ client.secretRotatedAt === null ? t('admin.common.none') : formatDate(client.secretRotatedAt) }}</dd>
            <dt class="text-muted-foreground">
              {{ t('admin.clients.table.created') }}
            </dt>
            <dd>{{ formatDate(client.createdAt) }}</dd>
          </dl>
        </CardContent>
      </Card>

      <AdminConfirmDialog
        :open="confirmDelete"
        :title="t('admin.clients.detail.deleteTitle')"
        :description="t('admin.clients.detail.deleteDescription')"
        :confirm-label="t('admin.clients.detail.delete')"
        :pending-label="t('admin.clients.detail.deleting')"
        :pending="deleteMutation.isPending.value"
        destructive
        @update:open="confirmDelete = $event"
        @confirm="remove"
      />

      <AdminConfirmDialog
        :open="confirmRotate"
        :title="t('admin.clients.detail.rotateTitle')"
        :description="t('admin.clients.detail.rotateDescription')"
        :confirm-label="t('admin.clients.detail.rotateSecret')"
        :pending-label="t('admin.clients.detail.rotating')"
        :pending="rotateMutation.isPending.value"
        @update:open="confirmRotate = $event"
        @confirm="rotate"
      />

      <AdminSecretDialog
        v-model:open="secretOpen"
        :title="t('admin.clients.secret.rotatedTitle', { name: client.name })"
        :description="t('admin.clients.secret.rotatedDescription')"
        :secret="secretValue"
        :warning="t('admin.clients.secret.warning')"
      />
    </template>
  </div>
</template>
