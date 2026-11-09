<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import { Button } from '@/components/ui/button'
import { DialogPanel } from '@/components/ui/dialog'

const props = defineProps<{
  open: boolean
  title: string
  description: string
  confirmLabel: string
  pendingLabel: string
  pending: boolean
  destructive?: boolean
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  confirm: []
}>()

const { t } = useI18n()
</script>

<template>
  <DialogPanel
    :open="props.open"
    :title="props.title"
    :description="props.description"
    :close-label="t('admin.common.close')"
    @update:open="emit('update:open', $event)"
  >
    <template #footer>
      <Button
        type="button"
        variant="outline"
        :disabled="props.pending"
        @click="emit('update:open', false)"
      >
        {{ t('admin.common.cancel') }}
      </Button>
      <Button
        type="button"
        :variant="props.destructive ? 'destructive' : 'default'"
        :disabled="props.pending"
        @click="emit('confirm')"
      >
        {{ props.pending ? props.pendingLabel : props.confirmLabel }}
      </Button>
    </template>
  </DialogPanel>
</template>
