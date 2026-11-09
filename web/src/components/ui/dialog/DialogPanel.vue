<script setup lang="ts">
import { DialogClose, DialogContent, DialogDescription, DialogOverlay, DialogPortal, DialogRoot, DialogTitle } from 'reka-ui'

import { cn } from '@/lib/utils'

const props = withDefaults(
  defineProps<{
    open: boolean
    title: string
    description?: string
    closeLabel?: string
    contentClass?: string
  }>(),
  {
    description: '',
    closeLabel: 'Close',
    contentClass: '',
  },
)

const emit = defineEmits<{
  'update:open': [value: boolean]
}>()
</script>

<template>
  <DialogRoot
    :open="props.open"
    @update:open="emit('update:open', $event)"
  >
    <DialogPortal>
      <DialogOverlay
        class="fixed inset-0 z-50 bg-black/50 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:animate-in data-[state=open]:fade-in-0"
      />
      <DialogContent
        :class="
          cn(
            'fixed left-1/2 top-1/2 z-50 max-h-[90vh] w-[calc(100%-2rem)] max-w-lg -translate-x-1/2 -translate-y-1/2 overflow-y-auto rounded-xl border bg-card p-6 text-card-foreground shadow-lg focus:outline-hidden',
            props.contentClass,
          )
        "
      >
        <div class="space-y-1.5 pr-8">
          <DialogTitle class="text-lg font-semibold tracking-tight">
            {{ props.title }}
          </DialogTitle>
          <DialogDescription
            v-if="props.description"
            class="text-sm text-muted-foreground"
          >
            {{ props.description }}
          </DialogDescription>
        </div>
        <DialogClose
          class="absolute right-4 top-4 rounded-md p-1 text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring"
          :aria-label="props.closeLabel"
        >
          <span aria-hidden="true">×</span>
        </DialogClose>
        <div class="mt-4">
          <slot />
        </div>
        <div
          v-if="$slots.footer"
          class="mt-6 flex flex-wrap items-center justify-end gap-2"
        >
          <slot name="footer" />
        </div>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
