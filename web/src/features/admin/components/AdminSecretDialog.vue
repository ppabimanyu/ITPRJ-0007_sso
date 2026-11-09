<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { DialogPanel } from '@/components/ui/dialog'

const props = defineProps<{
  open: boolean
  title: string
  description: string
  secret: string
  warning: string
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
}>()

const { t } = useI18n()
const copied = ref(false)
const copyFailed = ref(false)

watch(
  () => props.open,
  (open) => {
    if (open) {
      copied.value = false
      copyFailed.value = false
    }
  },
)

async function copy() {
  copyFailed.value = false
  try {
    await navigator.clipboard.writeText(props.secret)
    copied.value = true
  } catch {
    copied.value = false
    copyFailed.value = true
  }
}
</script>

<template>
  <DialogPanel
    :open="props.open"
    :title="props.title"
    :description="props.description"
    :close-label="t('admin.common.close')"
    @update:open="emit('update:open', $event)"
  >
    <div class="space-y-4">
      <Alert variant="destructive">
        <AlertDescription>{{ props.warning }}</AlertDescription>
      </Alert>
      <div class="flex items-start gap-2">
        <code
          class="min-w-0 flex-1 select-all break-all rounded-md border bg-muted/40 px-3 py-2 font-mono text-sm"
        >
          {{ props.secret }}
        </code>
        <Button
          type="button"
          variant="outline"
          size="sm"
          @click="copy"
        >
          {{ copied ? t('admin.common.copied') : t('admin.common.copy') }}
        </Button>
      </div>
      <Alert
        v-if="copyFailed"
        variant="destructive"
      >
        <AlertDescription>{{ t('admin.common.copyFailed') }}</AlertDescription>
      </Alert>
    </div>
    <template #footer>
      <Button
        type="button"
        @click="emit('update:open', false)"
      >
        {{ t('admin.common.savedClose') }}
      </Button>
    </template>
  </DialogPanel>
</template>
