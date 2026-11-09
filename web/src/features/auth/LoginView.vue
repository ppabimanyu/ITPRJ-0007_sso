<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import AuthShell from './AuthShell.vue'
import { useLogin } from './queries'
import { isLocalRedirect } from './redirect'
import { useAuthError } from './use-auth-error'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const { mutateAsync: login, isPending, error } = useLogin()
const { generalMessage, fieldErrors } = useAuthError()

const form = reactive({
  email: '',
  password: '',
  rememberMe: false,
})
const statusNotice = ref('')

const notice = computed(() => {
  if (route.query.notice === 'invite') {
    return t('auth.login.noticeInvite')
  }
  if (route.query.reset === 'success') {
    return t('auth.login.noticeReset')
  }
  return ''
})
const errorMessage = computed(() =>
  error.value === null
    ? ''
    : generalMessage(error.value, { unauthorized: 'auth.login.invalid' }),
)
const fields = computed(() => fieldErrors(error.value))
const emailError = computed(() => fields.value.email ?? '')
const passwordError = computed(() => fields.value.password ?? '')

async function submit() {
  statusNotice.value = ''
  try {
    const result = await login({
      email: form.email.trim(),
      password: form.password,
      rememberMe: form.rememberMe,
    })
    if (result.status !== 'authenticated') {
      statusNotice.value = t('auth.login.mfaUnavailable')
      return
    }
    const redirect = route.query.redirect
    await router.replace(isLocalRedirect(redirect) ? redirect : { name: 'account' })
  } catch {
    // The mutation error renders above the form.
  }
}
</script>

<template>
  <AuthShell
    :title="t('auth.login.title')"
    :description="t('auth.login.description')"
  >
    <Alert
      v-if="notice"
      variant="success"
    >
      <AlertDescription>{{ notice }}</AlertDescription>
    </Alert>
    <Alert
      v-if="statusNotice"
      variant="destructive"
    >
      <AlertDescription>{{ statusNotice }}</AlertDescription>
    </Alert>
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
        <Label for="login-email">{{ t('auth.login.email') }}</Label>
        <Input
          id="login-email"
          v-model="form.email"
          type="email"
          autocomplete="username"
          required
          :aria-invalid="emailError !== ''"
          :aria-describedby="emailError === '' ? undefined : 'login-email-error'"
        />
        <p
          v-if="emailError"
          id="login-email-error"
          role="alert"
          class="text-sm text-destructive"
        >
          {{ emailError }}
        </p>
      </div>

      <div class="space-y-2">
        <Label for="login-password">{{ t('auth.login.password') }}</Label>
        <Input
          id="login-password"
          v-model="form.password"
          type="password"
          autocomplete="current-password"
          required
          :aria-invalid="passwordError !== ''"
          :aria-describedby="passwordError === '' ? undefined : 'login-password-error'"
        />
        <p
          v-if="passwordError"
          id="login-password-error"
          role="alert"
          class="text-sm text-destructive"
        >
          {{ passwordError }}
        </p>
      </div>

      <div class="flex flex-wrap items-center justify-between gap-3">
        <label class="flex items-center gap-2 text-sm text-muted-foreground">
          <input
            v-model="form.rememberMe"
            type="checkbox"
            class="h-4 w-4 rounded border-input text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background"
          >
          {{ t('auth.login.remember') }}
        </label>
        <RouterLink
          to="/forgot-password"
          class="text-sm font-medium text-primary underline-offset-4 hover:underline"
        >
          {{ t('auth.login.forgot') }}
        </RouterLink>
      </div>

      <Button
        type="submit"
        class="w-full"
        :disabled="isPending"
      >
        {{ isPending ? t('auth.login.submitting') : t('auth.login.submit') }}
      </Button>
    </form>
  </AuthShell>
</template>
