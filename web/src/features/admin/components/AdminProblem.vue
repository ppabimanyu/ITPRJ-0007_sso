<script setup lang="ts">
import { computed } from 'vue'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'

import { useAdminError } from '../use-admin-error'

const props = defineProps<{
  error: unknown
}>()

const { describe } = useAdminError()
const problem = computed(() => (props.error === null ? null : describe(props.error)))
</script>

<template>
  <Alert
    v-if="problem"
    variant="destructive"
  >
    <AlertTitle>{{ problem.title }}</AlertTitle>
    <AlertDescription>
      <p>{{ problem.message }}</p>
      <p
        v-if="problem.detail"
        class="mt-1 break-words font-mono text-xs opacity-80"
      >
        {{ problem.detail }}
      </p>
    </AlertDescription>
  </Alert>
</template>
