<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import AuthShell from './AuthShell.vue'
import { useResetPassword } from './queries'
import { useAuthError } from './use-auth-error'

const { t } = useI18n()
const route = useRoute()
const { mutateAsync: reset, isPending, error } = useResetPassword()
const { generalMessage, fieldErrors } = useAuthError()

const token = computed(() => (typeof route.query.token === 'string' ? route.query.token : ''))
const form = reactive({ password: '', confirmation: '' })
const mismatch = ref('')
const done = ref(false)

const errorMessage = computed(() =>
  error.value === null
    ? ''
    : generalMessage(error.value, { badRequest: 'errors.invalidToken' }),
)
const fields = computed(() => fieldErrors(error.value))
const passwordError = computed(() => fields.value.new_password ?? '')

async function submit() {
  mismatch.value = ''
  if (form.password !== form.confirmation) {
    mismatch.value = t('auth.reset.mismatch')
    return
  }
  try {
    await reset({ token: token.value, newPassword: form.password })
    done.value = true
  } catch {
    // The mutation error renders above the form.
  }
}
</script>

<template>
  <AuthShell
    :title="t('auth.reset.title')"
    :description="t('auth.reset.description')"
  >
    <template v-if="done">
      <Alert>
        <AlertTitle>{{ t('auth.reset.successTitle') }}</AlertTitle>
        <AlertDescription>{{ t('auth.reset.success') }}</AlertDescription>
      </Alert>
      <Button
        as-child
        class="w-full"
      >
        <RouterLink to="/login">{{ t('auth.reset.goToLogin') }}</RouterLink>
      </Button>
    </template>

    <template v-else-if="token === ''">
      <Alert variant="destructive">
        <AlertTitle>{{ t('auth.reset.missingToken') }}</AlertTitle>
        <AlertDescription>{{ t('auth.reset.missingTokenHint') }}</AlertDescription>
      </Alert>
      <Button
        as-child
        variant="outline"
        class="w-full"
      >
        <RouterLink to="/forgot-password">{{ t('auth.forgot.title') }}</RouterLink>
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
          <Label for="reset-password">{{ t('auth.reset.newPassword') }}</Label>
          <Input
            id="reset-password"
            v-model="form.password"
            type="password"
            autocomplete="new-password"
            required
            :aria-invalid="passwordError !== ''"
            :aria-describedby="passwordError === '' ? undefined : 'reset-password-error'"
          />
          <p
            v-if="passwordError"
            id="reset-password-error"
            role="alert"
            class="text-sm text-destructive"
          >
            {{ passwordError }}
          </p>
        </div>

        <div class="space-y-2">
          <Label for="reset-confirmation">{{ t('auth.reset.confirmPassword') }}</Label>
          <Input
            id="reset-confirmation"
            v-model="form.confirmation"
            type="password"
            autocomplete="new-password"
            required
            :aria-invalid="mismatch !== ''"
            :aria-describedby="mismatch === '' ? undefined : 'reset-confirmation-error'"
          />
          <p
            v-if="mismatch"
            id="reset-confirmation-error"
            role="alert"
            class="text-sm text-destructive"
          >
            {{ mismatch }}
          </p>
        </div>

        <Button
          type="submit"
          class="w-full"
          :disabled="isPending"
        >
          {{ isPending ? t('auth.reset.submitting') : t('auth.reset.submit') }}
        </Button>
      </form>
    </template>
  </AuthShell>
</template>
