<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'

import AuthShell from './AuthShell.vue'
import { useVerifyEmail } from './queries'
import { useAuthError } from './use-auth-error'

const { t } = useI18n()
const route = useRoute()
const { mutate, isPending, isSuccess, error } = useVerifyEmail()
const { generalMessage } = useAuthError()

const token = computed(() => (typeof route.query.token === 'string' ? route.query.token : ''))
const errorMessage = computed(() =>
  error.value === null
    ? ''
    : generalMessage(error.value, { badRequest: 'errors.invalidToken' }),
)

onMounted(() => {
  if (token.value !== '') {
    mutate(token.value)
  }
})
</script>

<template>
  <AuthShell
    :title="t('auth.verify.title')"
    :description="t('auth.verify.description')"
  >
    <Alert
      v-if="token === ''"
      variant="destructive"
    >
      <AlertTitle>{{ t('auth.verify.missingToken') }}</AlertTitle>
      <AlertDescription>{{ t('auth.verify.missingTokenHint') }}</AlertDescription>
    </Alert>

    <Alert
      v-else-if="errorMessage"
      variant="destructive"
    >
      <AlertTitle>{{ t('auth.verify.errorTitle') }}</AlertTitle>
      <AlertDescription>{{ errorMessage }}</AlertDescription>
    </Alert>

    <Alert
      v-else-if="isSuccess"
      variant="success"
    >
      <AlertTitle>{{ t('auth.verify.successTitle') }}</AlertTitle>
      <AlertDescription>{{ t('auth.verify.success') }}</AlertDescription>
    </Alert>

    <p
      v-else
      class="text-sm text-muted-foreground"
    >
      {{ isPending ? t('auth.verify.verifying') : t('common.loading') }}
    </p>

    <Button
      as-child
      class="w-full"
      variant="outline"
    >
      <RouterLink to="/login">{{ t('auth.verify.goToLogin') }}</RouterLink>
    </Button>
  </AuthShell>
</template>
