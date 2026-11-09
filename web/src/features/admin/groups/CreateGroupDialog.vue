<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { Button } from '@/components/ui/button'
import { DialogPanel } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'

import AdminProblem from '../components/AdminProblem.vue'
import { useCreateGroup } from '../queries'
import { useAdminError } from '../use-admin-error'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
}>()

const { t } = useI18n()
const { mutateAsync: create, isPending, error, reset } = useCreateGroup()
const { fieldErrors } = useAdminError()
const fields = computed(() => fieldErrors(error.value))
const form = reactive({ name: '', description: '' })

watch(
  () => props.open,
  (open) => {
    if (!open) {
      return
    }
    form.name = ''
    form.description = ''
    reset()
  },
)

async function submit() {
  try {
    await create({
      name: form.name.trim(),
      description: form.description.trim() === '' ? null : form.description.trim(),
      idempotencyKey: crypto.randomUUID(),
    })
    emit('update:open', false)
  } catch {
    // The mutation error renders above the form.
  }
}
</script>

<template>
  <DialogPanel
    :open="props.open"
    :title="t('admin.groups.create.title')"
    :description="t('admin.groups.create.description')"
    :close-label="t('admin.common.close')"
    @update:open="emit('update:open', $event)"
  >
    <form
      class="space-y-4"
      novalidate
      @submit.prevent="submit"
    >
      <AdminProblem :error="error" />
      <div class="space-y-2">
        <Label for="create-group-name">{{ t('admin.groups.create.name') }}</Label>
        <Input
          id="create-group-name"
          v-model="form.name"
          :aria-invalid="fields.name !== undefined"
        />
        <p class="text-xs text-muted-foreground">
          {{ t('admin.groups.create.nameHint') }}
        </p>
        <p
          v-if="fields.name"
          role="alert"
          class="text-sm text-destructive"
        >
          {{ fields.name }}
        </p>
      </div>
      <div class="space-y-2">
        <Label for="create-group-description">{{ t('admin.groups.create.descriptionLabel') }}</Label>
        <Textarea
          id="create-group-description"
          v-model="form.description"
          :aria-invalid="fields.description !== undefined"
        />
        <p
          v-if="fields.description"
          role="alert"
          class="text-sm text-destructive"
        >
          {{ fields.description }}
        </p>
      </div>
      <div class="flex flex-wrap justify-end gap-2">
        <Button
          type="button"
          variant="outline"
          :disabled="isPending"
          @click="emit('update:open', false)"
        >
          {{ t('admin.common.cancel') }}
        </Button>
        <Button
          type="submit"
          :disabled="isPending"
        >
          {{ isPending ? t('admin.groups.create.submitting') : t('admin.groups.create.submit') }}
        </Button>
      </div>
    </form>
  </DialogPanel>
</template>
