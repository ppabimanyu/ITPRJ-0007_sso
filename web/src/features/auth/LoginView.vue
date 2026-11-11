<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQueryClient } from '@tanstack/vue-query'
import { useRoute, useRouter } from 'vue-router'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { isApiError } from '@/lib/http'
import { parseAuthorizeRequestId, type AuthorizeRequestInfo } from '@/lib/oidc-api'
import { oidcKeys } from '@/lib/query-keys'

import AuthorizeRequestError from './AuthorizeRequestError.vue'
import AuthorizeRequestSummary from './AuthorizeRequestSummary.vue'
import AuthShell from './AuthShell.vue'
import { useAuthorizeRequest, useCompleteAuthorize } from './oidc-queries'
import { useLogin, useVerifyMfa } from './queries'
import { isLocalRedirect } from './redirect'
import { useAuthError } from './use-auth-error'
import { useAuthorizeError } from './use-authorize-error'

type ChallengeMode = 'totp' | 'recovery'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const queryClient = useQueryClient()
const { mutateAsync: login, isPending, error } = useLogin()
const {
  mutateAsync: verifyChallenge,
  isPending: verifying,
  error: verifyError,
  reset: resetVerify,
} = useVerifyMfa()
const { mutateAsync: complete, isPending: completing, error: completeError } =
  useCompleteAuthorize()
const { generalMessage, fieldErrors } = useAuthError()
const { message: authorizeMessage } = useAuthorizeError()

const hasRequestQuery = computed(() => route.query.request_id !== undefined)
const requestId = computed(() => parseAuthorizeRequestId(route.query.request_id))
const authorizeQuery = useAuthorizeRequest(requestId)
const authRequest = computed<AuthorizeRequestInfo | null>(
  () => authorizeQuery.data.value ?? null,
)

const form = reactive({
  email: '',
  password: '',
  rememberMe: false,
})
const statusNotice = ref('')
const mfaToken = ref<string | null>(null)
const challengeMode = ref<ChallengeMode>('totp')
const challengeForm = reactive({
  code: '',
  recoveryCode: '',
})
const failedAttempts = ref(0)

const challengeCode = computed({
  get: () => challengeForm.code,
  set: (value: string) => {
    challengeForm.code = value.replace(/\D/g, '').slice(0, 6)
  },
})
const challengeExpired = computed(() => failedAttempts.value >= 5)
const canSubmitChallenge = computed(() =>
  challengeMode.value === 'totp'
    ? challengeCode.value.length === 6
    : challengeForm.recoveryCode.trim() !== '',
)
const challengeErrorMessage = computed(() => {
  if (verifyError.value === null) {
    return ''
  }
  if (!isApiError(verifyError.value)) {
    return t('errors.general')
  }
  if (verifyError.value.status === 401) {
    return challengeExpired.value ? t('auth.mfa.expired') : t('auth.mfa.invalid')
  }
  return generalMessage(verifyError.value, { badRequest: 'auth.mfa.invalid' })
})
const shellTitle = computed(() =>
  mfaToken.value === null ? t('auth.login.title') : t('auth.mfa.title'),
)

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
const requestError = computed(() => {
  if (!hasRequestQuery.value) {
    return ''
  }
  if (requestId.value === null) {
    return t('auth.authorize.invalid')
  }
  if (authorizeQuery.isError.value) {
    return authorizeMessage(authorizeQuery.error.value)
  }
  return ''
})
const finishError = computed(() =>
  completeError.value === null ? '' : authorizeMessage(completeError.value),
)
const waiting = computed(
  () =>
    hasRequestQuery.value &&
    requestId.value !== null &&
    authRequest.value === null &&
    !authorizeQuery.isError.value,
)
const description = computed(() => {
  if (authRequest.value === null) {
    return t('auth.login.description')
  }
  return t('auth.login.requestDescription', { client: authRequest.value.clientName })
})
const shellDescription = computed(() =>
  mfaToken.value === null ? description.value : t('auth.mfa.description'),
)
const submitLabel = computed(() => {
  if (completing.value) {
    return t('auth.authorize.completing')
  }
  return isPending.value ? t('auth.login.submitting') : t('auth.login.submit')
})

watch(
  authRequest,
  (info) => {
    if (info?.loginHint !== null && info?.loginHint !== undefined && form.email === '') {
      form.email = info.loginHint
    }
  },
  { immediate: true },
)

async function finishRequest(info: AuthorizeRequestInfo) {
  if (info.consentRequired) {
    queryClient.removeQueries({ queryKey: oidcKeys.authorizeRequest(info.requestId) })
    await router.replace({ name: 'consent', query: { request_id: info.requestId } })
    return
  }
  const redirectTo = await complete(info.requestId)
  window.location.assign(redirectTo)
}

async function continueAfterAuthentication() {
  if (authRequest.value !== null) {
    await finishRequest(authRequest.value)
    return
  }
  const redirect = route.query.redirect
  await router.replace(isLocalRedirect(redirect) ? redirect : { name: 'account' })
}

async function continueRequest() {
  const info = authRequest.value
  if (info === null) {
    return
  }
  try {
    await finishRequest(info)
  } catch {
    // The mutation error renders above the action.
  }
}

function nextPageQuery(): Record<string, string> {
  const query: Record<string, string> = {}
  const id = authRequest.value?.requestId ?? requestId.value
  if (id !== null) {
    query.request_id = id
  }
  if (typeof route.query.redirect === 'string') {
    query.redirect = route.query.redirect
  }
  return query
}

async function focusChallengeCode() {
  await nextTick()
  const id = challengeMode.value === 'totp' ? 'login-mfa-code' : 'login-mfa-recovery'
  document.getElementById(id)?.focus()
}

async function submitChallenge() {
  const token = mfaToken.value
  if (token === null) {
    return
  }
  statusNotice.value = ''
  try {
    if (challengeMode.value === 'totp') {
      await verifyChallenge({ mfaToken: token, code: challengeCode.value })
    } else {
      await verifyChallenge({
        mfaToken: token,
        recoveryCode: challengeForm.recoveryCode.trim(),
      })
    }
    await continueAfterAuthentication()
  } catch (error) {
    if (isApiError(error) && error.status === 401) {
      failedAttempts.value += 1
    }
  }
}

function backToLogin() {
  mfaToken.value = null
  challengeMode.value = 'totp'
  challengeForm.code = ''
  challengeForm.recoveryCode = ''
  failedAttempts.value = 0
  statusNotice.value = ''
  resetVerify()
}

async function switchChallengeMode(mode: ChallengeMode) {
  if (challengeMode.value === mode) {
    return
  }
  challengeMode.value = mode
  statusNotice.value = ''
  resetVerify()
  await focusChallengeCode()
}

async function submit() {
  statusNotice.value = ''
  try {
    const result = await login({
      email: form.email.trim(),
      password: form.password,
      rememberMe: form.rememberMe,
      requestId: authRequest.value?.requestId ?? requestId.value,
    })
    if (result.status === 'mfa_required') {
      if (result.mfaToken === null) {
        statusNotice.value = t('auth.mfa.unavailable')
        return
      }
      form.password = ''
      mfaToken.value = result.mfaToken
      failedAttempts.value = 0
      await focusChallengeCode()
      return
    }
    if (result.status === 'mfa_enrollment_required') {
      form.password = ''
      await router.push({ name: 'mfa-enroll', query: nextPageQuery() })
      return
    }
    await continueAfterAuthentication()
  } catch {
    // The mutation error renders above the form.
  }
}
</script>

<template>
  <AuthorizeRequestError
    v-if="requestError"
    :message="requestError"
  />
  <AuthShell
    v-else
    :title="shellTitle"
    :description="shellDescription"
  >
    <Alert
      v-if="notice"
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
    <Alert
      v-if="finishError"
      variant="destructive"
    >
      <AlertTitle>{{ t('errors.title') }}</AlertTitle>
      <AlertDescription>{{ finishError }}</AlertDescription>
    </Alert>

    <p
      v-if="waiting"
      class="text-sm text-muted-foreground"
    >
      {{ t('auth.authorize.loading') }}
    </p>

    <template v-else>
      <AuthorizeRequestSummary
        v-if="authRequest"
        :client-name="authRequest.clientName"
        :scopes="authRequest.scopes"
      />

      <form
        v-if="mfaToken !== null"
        class="space-y-5"
        novalidate
        @submit.prevent="submitChallenge"
      >
        <Alert
          v-if="challengeErrorMessage"
          variant="destructive"
        >
          <AlertTitle>{{ t('auth.mfa.errorTitle') }}</AlertTitle>
          <AlertDescription id="login-mfa-error">
            {{ challengeErrorMessage }}
          </AlertDescription>
        </Alert>

        <template v-if="challengeMode === 'totp'">
          <div class="space-y-2">
            <Label for="login-mfa-code">{{ t('auth.mfa.code') }}</Label>
            <Input
              id="login-mfa-code"
              v-model="challengeCode"
              inputmode="numeric"
              autocomplete="one-time-code"
              maxlength="6"
              pattern="[0-9]*"
              required
              :aria-invalid="challengeErrorMessage !== ''"
              :aria-describedby="challengeErrorMessage === '' ? undefined : 'login-mfa-error'"
            />
            <p class="text-xs text-muted-foreground">
              {{ t('auth.mfa.codeHint') }}
            </p>
          </div>
          <Button
            type="button"
            variant="link"
            class="h-auto p-0"
            @click="switchChallengeMode('recovery')"
          >
            {{ t('auth.mfa.useRecovery') }}
          </Button>
        </template>

        <template v-else>
          <div class="space-y-2">
            <Label for="login-mfa-recovery">{{ t('auth.mfa.recoveryCode') }}</Label>
            <Input
              id="login-mfa-recovery"
              v-model="challengeForm.recoveryCode"
              autocomplete="one-time-code"
              autocapitalize="characters"
              spellcheck="false"
              required
              :aria-invalid="challengeErrorMessage !== ''"
              :aria-describedby="challengeErrorMessage === '' ? undefined : 'login-mfa-error'"
            />
            <p class="text-xs text-muted-foreground">
              {{ t('auth.mfa.recoveryHint') }}
            </p>
          </div>
          <Button
            type="button"
            variant="link"
            class="h-auto p-0"
            @click="switchChallengeMode('totp')"
          >
            {{ t('auth.mfa.useTotp') }}
          </Button>
        </template>

        <Button
          type="submit"
          class="w-full"
          :disabled="verifying || !canSubmitChallenge"
        >
          {{ verifying ? t('auth.mfa.verifying') : t('auth.mfa.verify') }}
        </Button>
        <Button
          type="button"
          variant="outline"
          class="w-full"
          :disabled="verifying"
          @click="backToLogin"
        >
          {{ t('auth.mfa.backToLogin') }}
        </Button>
      </form>

      <template v-else-if="authRequest && !authRequest.loginRequired">
        <p class="text-sm text-muted-foreground">
          {{ t('auth.authorize.signedIn') }}
        </p>
        <Button
          class="w-full"
          :disabled="completing"
          @click="continueRequest"
        >
          {{
            completing
              ? t('auth.authorize.continuing')
              : t('auth.authorize.continue')
          }}
        </Button>
      </template>

      <form
        v-else
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
              class="h-4 w-4 rounded border-input text-primary focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background"
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
          :disabled="isPending || completing"
        >
          {{ submitLabel }}
        </Button>
      </form>
    </template>
  </AuthShell>
</template>
