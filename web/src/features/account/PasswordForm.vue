<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import { useChangePassword } from '../auth/queries'
import { useAuthError } from '../auth/use-auth-error'

const { t } = useI18n()
const { mutateAsync: changePassword, isPending, error } = useChangePassword()
const { generalMessage, fieldErrors } = useAuthError()

const form = reactive({
  currentPassword: '',
  newPassword: '',
  confirmation: '',
})
const mismatch = ref('')
const saved = ref(false)

const errorMessage = computed(() =>
  error.value === null
    ? ''
    : generalMessage(error.value, { unauthorized: 'account.password.currentIncorrect' }),
)
const fields = computed(() => fieldErrors(error.value))
const currentError = computed(() => fields.value.current_password ?? '')
const newError = computed(() => fields.value.new_password ?? '')

async function submit() {
  mismatch.value = ''
  saved.value = false
  if (form.newPassword !== form.confirmation) {
    mismatch.value = t('account.password.mismatch')
    return
  }
  try {
    await changePassword({
      currentPassword: form.currentPassword,
      newPassword: form.newPassword,
    })
    form.currentPassword = ''
    form.newPassword = ''
    form.confirmation = ''
    saved.value = true
  } catch {
    // The mutation error renders above the form.
  }
}
</script>

<template>
  <section class="space-y-4">
    <div class="space-y-1">
      <h2 class="text-lg font-semibold tracking-tight">
        {{ t('account.password.title') }}
      </h2>
      <p class="text-sm text-muted-foreground">
        {{ t('account.password.description') }}
      </p>
    </div>

    <Alert
      v-if="saved"
      variant="success"
    >
      <AlertDescription>{{ t('account.password.saved') }}</AlertDescription>
    </Alert>
    <Alert
      v-if="errorMessage"
      variant="destructive"
    >
      <AlertTitle>{{ t('errors.title') }}</AlertTitle>
      <AlertDescription>{{ errorMessage }}</AlertDescription>
    </Alert>

    <form
      class="space-y-4"
      novalidate
      @submit.prevent="submit"
    >
      <div class="space-y-2">
        <Label for="password-current">{{ t('account.password.current') }}</Label>
        <Input
          id="password-current"
          v-model="form.currentPassword"
          type="password"
          autocomplete="current-password"
          required
          :aria-invalid="currentError !== ''"
          :aria-describedby="currentError === '' ? undefined : 'password-current-error'"
        />
        <p
          v-if="currentError"
          id="password-current-error"
          role="alert"
          class="text-sm text-destructive"
        >
          {{ currentError }}
        </p>
      </div>

      <div class="space-y-2">
        <Label for="password-new">{{ t('account.password.new') }}</Label>
        <Input
          id="password-new"
          v-model="form.newPassword"
          type="password"
          autocomplete="new-password"
          required
          :aria-invalid="newError !== ''"
          :aria-describedby="newError === '' ? undefined : 'password-new-error'"
        />
        <p
          v-if="newError"
          id="password-new-error"
          role="alert"
          class="text-sm text-destructive"
        >
          {{ newError }}
        </p>
      </div>

      <div class="space-y-2">
        <Label for="password-confirmation">{{ t('account.password.confirm') }}</Label>
        <Input
          id="password-confirmation"
          v-model="form.confirmation"
          type="password"
          autocomplete="new-password"
          required
          :aria-invalid="mismatch !== ''"
          :aria-describedby="mismatch === '' ? undefined : 'password-confirmation-error'"
        />
        <p
          v-if="mismatch"
          id="password-confirmation-error"
          role="alert"
          class="text-sm text-destructive"
        >
          {{ mismatch }}
        </p>
      </div>

      <Button
        type="submit"
        :disabled="isPending"
      >
        {{ isPending ? t('account.password.submitting') : t('account.password.submit') }}
      </Button>
    </form>
  </section>
</template>
