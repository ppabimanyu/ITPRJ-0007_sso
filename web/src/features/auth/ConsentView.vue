<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { parseAuthorizeRequestId } from '@/lib/oidc-api'

import AuthorizeRequestError from './AuthorizeRequestError.vue'
import AuthorizeRequestSummary from './AuthorizeRequestSummary.vue'
import AuthShell from './AuthShell.vue'
import { useAuthorizeRequest, useGrantConsent } from './oidc-queries'
import { useSession } from './queries'
import { useAuthorizeError } from './use-authorize-error'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const { mutateAsync: grant, isPending, error } = useGrantConsent()
const { message: authorizeMessage } = useAuthorizeError()
const sessionQuery = useSession()

const requestId = computed(() => parseAuthorizeRequestId(route.query.request_id))
const authorizeQuery = useAuthorizeRequest(requestId)
const authRequest = computed(() => authorizeQuery.data.value ?? null)
const requestError = computed(() => {
  if (requestId.value === null) {
    return t('auth.authorize.invalid')
  }
  if (authorizeQuery.isError.value) {
    return authorizeMessage(authorizeQuery.error.value)
  }
  return ''
})
const waiting = computed(
  () =>
    requestId.value !== null &&
    (authRequest.value === null || sessionQuery.isPending.value) &&
    !authorizeQuery.isError.value,
)
const needsLogin = computed(() => {
  if (sessionQuery.isSuccess.value && sessionQuery.data.value === null) {
    return true
  }
  return (
    authorizeQuery.isFetchedAfterMount.value &&
    authRequest.value?.loginRequired === true
  )
})
const redirecting = ref(false)
const errorMessage = computed(() =>
  error.value === null ? '' : authorizeMessage(error.value),
)
const description = computed(() => {
  if (authRequest.value === null) {
    return t('auth.consent.genericDescription')
  }
  return t('auth.consent.description', { client: authRequest.value.clientName })
})

watch(
  needsLogin,
  (value) => {
    if (!value) {
      return
    }
    redirecting.value = true
    void router.replace({
      name: 'login',
      query: { request_id: requestId.value ?? undefined },
    })
  },
  { immediate: true },
)

async function approve() {
  const info = authRequest.value
  if (info === null) {
    return
  }
  try {
    const redirectTo = await grant({ requestId: info.requestId, scopes: info.scopes })
    window.location.assign(redirectTo)
  } catch {
    // The mutation error renders above the actions.
  }
}

async function cancel() {
  await router.replace({ name: 'home' })
}
</script>

<template>
  <AuthorizeRequestError
    v-if="requestError"
    :message="requestError"
  />
  <AuthShell
    v-else
    :title="t('auth.consent.title')"
    :description="description"
  >
    <p
      v-if="waiting || redirecting"
      class="text-sm text-muted-foreground"
    >
      {{ redirecting ? t('auth.consent.redirecting') : t('common.loading') }}
    </p>

    <template v-else-if="authRequest">
      <AuthorizeRequestSummary
        :client-name="authRequest.clientName"
        :scopes="authRequest.scopes"
      />

      <p class="text-sm text-muted-foreground">
        {{ t('auth.consent.scopesNote') }}
      </p>

      <Alert
        v-if="errorMessage"
        variant="destructive"
      >
        <AlertTitle>{{ t('errors.title') }}</AlertTitle>
        <AlertDescription>{{ errorMessage }}</AlertDescription>
      </Alert>

      <div class="flex flex-col-reverse gap-3 sm:flex-row">
        <Button
          variant="outline"
          class="sm:flex-1"
          :disabled="isPending"
          @click="cancel"
        >
          {{ t('auth.consent.cancel') }}
        </Button>
        <Button
          class="sm:flex-[2]"
          :disabled="isPending"
          @click="approve"
        >
          {{ isPending ? t('auth.consent.approving') : t('auth.consent.approve') }}
        </Button>
      </div>
    </template>
  </AuthShell>
</template>
