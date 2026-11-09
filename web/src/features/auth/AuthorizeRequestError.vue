<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'

import AuthShell from './AuthShell.vue'

defineProps<{
  message: string
}>()

const { t } = useI18n()
</script>

<template>
  <AuthShell
    :title="t('auth.authorize.errorTitle')"
    :description="t('auth.authorize.errorDescription')"
  >
    <Alert variant="destructive">
      <AlertTitle>{{ t('errors.title') }}</AlertTitle>
      <AlertDescription>{{ message }}</AlertDescription>
    </Alert>
    <Button
      as-child
      variant="outline"
      class="w-full"
    >
      <RouterLink to="/">
        {{ t('auth.authorize.home') }}
      </RouterLink>
    </Button>
  </AuthShell>
</template>
