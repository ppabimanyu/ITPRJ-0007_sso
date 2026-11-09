<script setup lang="ts">
import { computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import type { AdminClientWithSecret } from '@/lib/admin/types'

import AdminProblem from '../components/AdminProblem.vue'
import { useCreateClient } from '../queries'
import { useAdminError } from '../use-admin-error'
import ClientForm from './ClientForm.vue'
import type { ClientFormValues } from './client-form'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  created: [client: AdminClientWithSecret]
}>()

const { t } = useI18n()
const { mutateAsync: create, isPending, error, reset } = useCreateClient()
const { fieldErrors } = useAdminError()
const fields = computed(() => fieldErrors(error.value))

watch(
  () => props.open,
  (open) => {
    if (open) {
      reset()
    }
  },
)

async function submit(values: ClientFormValues) {
  try {
    const client = await create({ ...values, idempotencyKey: crypto.randomUUID() })
    emit('created', client)
    emit('update:open', false)
  } catch {
    // The mutation error renders above the form.
  }
}
</script>

<template>
  <Dialog
    :open="props.open"
    @update:open="emit('update:open', $event)"
  >
    <DialogContent class="max-h-[90vh] overflow-y-auto sm:max-w-2xl">
      <DialogHeader>
        <DialogTitle>{{ t('admin.clients.create.title') }}</DialogTitle>
        <DialogDescription>{{ t('admin.clients.create.description') }}</DialogDescription>
      </DialogHeader>
      <div class="space-y-4">
        <AdminProblem :error="error" />
        <ClientForm
          :initial="null"
          :pending="isPending"
          :field-errors="fields"
          :submit-label="t('admin.clients.create.submit')"
          :pending-label="t('admin.clients.create.submitting')"
          :allow-type-change="true"
          :show-disabled="false"
          @submit="submit"
          @cancel="emit('update:open', false)"
        />
      </div>
    </DialogContent>
  </Dialog>
</template>
