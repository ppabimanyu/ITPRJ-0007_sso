<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import AuthShell from './AuthShell.vue'
import { useForgotPassword } from './queries'
import { useAuthError } from './use-auth-error'

const { t } = useI18n()
const { mutateAsync: requestReset, isPending, error } = useForgotPassword()
const { generalMessage, fieldErrors } = useAuthError()

const form = reactive({ email: '' })
const sent = ref(false)
const errorMessage = computed(() => (error.value === null ? '' : generalMessage(error.value)))
const emailError = computed(() => fieldErrors(error.value).email ?? '')

async function submit() {
  try {
    await requestReset(form.email.trim())
    sent.value = true
  } catch {
    // The mutation error renders above the form.
  }
}
</script>

<template>
  <AuthShell
    :title="t('auth.forgot.title')"
    :description="t('auth.forgot.description')"
  >
    <template v-if="sent">
      <Alert variant="success">
        <AlertTitle>{{ t('auth.forgot.successTitle') }}</AlertTitle>
        <AlertDescription>{{ t('auth.forgot.success') }}</AlertDescription>
      </Alert>
      <Button
        as-child
        variant="outline"
        class="w-full"
      >
        <RouterLink to="/login">{{ t('auth.forgot.backToLogin') }}</RouterLink>
      </Button>
    </template>

    <template v-else>
      <Alert
        v-if="errorMessage"
        variant="destructive"
      >
        <AlertTitle>{{ t('errors.title') }}</AlertTitle>
        <AlertDescription>{{ errorMessage }}</AlertDescription>
      </Alert>

      <form
        class="space-y-5"
        novalidate
        @submit.prevent="submit"
      >
        <div class="space-y-2">
          <Label for="forgot-email">{{ t('auth.forgot.email') }}</Label>
          <Input
            id="forgot-email"
            v-model="form.email"
            type="email"
            autocomplete="username"
            required
            :aria-invalid="emailError !== ''"
            :aria-describedby="emailError === '' ? undefined : 'forgot-email-error'"
          />
          <p
            v-if="emailError"
            id="forgot-email-error"
            role="alert"
            class="text-sm text-destructive"
          >
            {{ emailError }}
          </p>
        </div>

        <Button
          type="submit"
          class="w-full"
          :disabled="isPending"
        >
          {{ isPending ? t('auth.forgot.submitting') : t('auth.forgot.submit') }}
        </Button>
        <RouterLink
          to="/login"
          class="block text-center text-sm font-medium text-primary underline-offset-4 hover:underline"
        >
          {{ t('auth.forgot.backToLogin') }}
        </RouterLink>
      </form>
    </template>
  </AuthShell>
</template>
