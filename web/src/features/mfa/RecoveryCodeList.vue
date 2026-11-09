<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'

defineProps<{
  codes: string[]
  title: string
  description: string
  warning: string
}>()

const { t } = useI18n()
const copied = ref(false)
const copyFailed = ref(false)

async function copy(codes: string[]) {
  copyFailed.value = false
  try {
    await navigator.clipboard.writeText(codes.join('\n'))
    copied.value = true
  } catch {
    copied.value = false
    copyFailed.value = true
  }
}
</script>

<template>
  <div class="space-y-4">
    <Alert>
      <AlertTitle>{{ title }}</AlertTitle>
      <AlertDescription>{{ description }}</AlertDescription>
    </Alert>

    <ol class="grid grid-cols-1 gap-1 rounded-md border bg-muted/40 p-4 font-mono text-sm sm:grid-cols-2">
      <li
        v-for="(code, index) in codes"
        :key="code"
        class="select-all"
      >
        <span class="mr-2 inline-block w-5 text-right text-muted-foreground">{{ index + 1 }}.</span>
        <span>{{ code }}</span>
      </li>
    </ol>

    <Alert
      v-if="copyFailed"
      variant="destructive"
    >
      <AlertDescription>{{ t('common.copyFailed') }}</AlertDescription>
    </Alert>

    <div class="flex flex-wrap items-center gap-3">
      <Button
        type="button"
        variant="outline"
        @click="copy(codes)"
      >
        {{ copied ? t('common.copied') : t('common.copy') }}
      </Button>
      <slot name="actions" />
    </div>

    <p class="text-xs text-muted-foreground">
      {{ warning }}
    </p>
  </div>
</template>
