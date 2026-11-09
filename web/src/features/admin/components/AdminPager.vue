<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import { Button } from '@/components/ui/button'

const props = defineProps<{
  nextAfter: string | null
  hasPrevious: boolean
  pending: boolean
}>()

const emit = defineEmits<{
  next: []
  previous: []
}>()

const { t } = useI18n()
</script>

<template>
  <div class="flex flex-wrap items-center justify-end gap-2">
    <Button
      type="button"
      variant="outline"
      size="sm"
      :disabled="!props.hasPrevious || props.pending"
      @click="emit('previous')"
    >
      {{ t('admin.common.previous') }}
    </Button>
    <Button
      type="button"
      variant="outline"
      size="sm"
      :disabled="props.nextAfter === null || props.pending"
      @click="emit('next')"
    >
      {{ t('admin.common.next') }}
    </Button>
  </div>
</template>
