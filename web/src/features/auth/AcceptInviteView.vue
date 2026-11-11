<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import AuthShell from './AuthShell.vue'
import { useAcceptInvite } from './queries'
import { useAuthError } from './use-auth-error'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const { mutateAsync: accept, isPending, error } = useAcceptInvite()
const { generalMessage, fieldErrors } = useAuthError()

const token = computed(() => (typeof route.query.token === 'string' ? route.query.token : ''))
const form = reactive({ password: '', confirmation: '' })
const mismatch = ref('')

const errorMessage = computed(() =>
  error.value === null
    ? ''
    : generalMessage(error.value, { badRequest: 'errors.invalidToken' }),
)
const passwordError = computed(() => fieldErrors(error.value).new_password ?? '')

async function submit() {
  mismatch.value = ''
  if (form.password !== form.confirmation) {
    mismatch.value = t('auth.invite.mismatch')
    return
  }
  try {
    await accept({ token: token.value, password: form.password })
    await router.replace({ name: 'login', query: { notice: 'invite' } })
  } catch {
    // The mutation error renders above the form.
  }
}
</script>

<template>
  <AuthShell
    :title="t('auth.invite.title')"
    :description="t('auth.invite.description')"
  >
    <template v-if="token === ''">
      <Alert variant="destructive">
        <AlertTitle>{{ t('auth.invite.missingToken') }}</AlertTitle>
        <AlertDescription>{{ t('auth.invite.missingTokenHint') }}</AlertDescription>
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
          <Label for="invite-password">{{ t('auth.invite.password') }}</Label>
          <Input
            id="invite-password"
            v-model="form.password"
            type="password"
            autocomplete="new-password"
            required
            :aria-invalid="passwordError !== ''"
            :aria-describedby="passwordError === '' ? undefined : 'invite-password-error'"
          />
          <p
            v-if="passwordError"
            id="invite-password-error"
            role="alert"
            class="text-sm text-destructive"
          >
            {{ passwordError }}
          </p>
        </div>

        <div class="space-y-2">
          <Label for="invite-confirmation">{{ t('auth.invite.confirmPassword') }}</Label>
          <Input
            id="invite-confirmation"
            v-model="form.confirmation"
            type="password"
            autocomplete="new-password"
            required
            :aria-invalid="mismatch !== ''"
            :aria-describedby="mismatch === '' ? undefined : 'invite-confirmation-error'"
          />
          <p
            v-if="mismatch"
            id="invite-confirmation-error"
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
          {{ isPending ? t('auth.invite.submitting') : t('auth.invite.submit') }}
        </Button>
      </form>
    </template>
  </AuthShell>
</template>
