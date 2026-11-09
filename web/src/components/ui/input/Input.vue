<script setup lang="ts">
import { ref, type HTMLAttributes } from 'vue'

import { cn } from '@/lib/utils'

const props = defineProps<{
  class?: HTMLAttributes['class']
}>()

const model = defineModel<string>({ default: '' })
const input = ref<HTMLInputElement | null>(null)

defineExpose({
  focus: () => input.value?.focus(),
})
</script>

<template>
  <input
    ref="input"
    v-model="model"
    :class="
      cn(
        'flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm transition-colors placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:cursor-not-allowed disabled:opacity-50',
        props.class,
      )
    "
  >
</template>
