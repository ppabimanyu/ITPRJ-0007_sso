<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'

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
  <AlertDialog
    :open="props.open"
    @update:open="emit('update:open', $event)"
  >
    <AlertDialogContent>
      <AlertDialogHeader>
        <AlertDialogTitle>{{ props.title }}</AlertDialogTitle>
        <AlertDialogDescription>{{ props.description }}</AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel :disabled="props.pending">
          {{ t('admin.common.cancel') }}
        </AlertDialogCancel>
        <Button
          type="button"
          :variant="props.destructive ? 'destructive' : 'default'"
          :disabled="props.pending"
          @click="emit('confirm')"
        >
          {{ props.pending ? props.pendingLabel : props.confirmLabel }}
        </Button>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>
</template>
