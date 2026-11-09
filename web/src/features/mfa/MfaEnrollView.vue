<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQueryClient } from '@tanstack/vue-query'
import { useRoute, useRouter } from 'vue-router'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { fetchAuthorizeRequest, parseAuthorizeRequestId } from '@/lib/oidc-api'
import { oidcKeys } from '@/lib/query-keys'

import AuthorizeRequestSummary from '../auth/AuthorizeRequestSummary.vue'
import AuthShell from '../auth/AuthShell.vue'
import { useAuthorizeRequest, useCompleteAuthorize } from '../auth/oidc-queries'
import { isLocalRedirect } from '../auth/redirect'
import { useAuthorizeError } from '../auth/use-authorize-error'
import MfaEnrollmentFlow from './MfaEnrollmentFlow.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const queryClient = useQueryClient()
const { mutateAsync: completeAuthorize, error: completeError } = useCompleteAuthorize()
const { message: authorizeMessage } = useAuthorizeError()

const requestId = computed(() => parseAuthorizeRequestId(route.query.request_id))
const authorizeQuery = useAuthorizeRequest(requestId)
const finishError = ref('')
const completeMessage = computed(() =>
  completeError.value === null ? '' : authorizeMessage(completeError.value),
)

const alertMessage = computed(() => finishError.value || completeMessage.value)

async function finish() {
  finishError.value = ''
  try {
    const id = requestId.value
    if (id !== null) {
      const info = await fetchAuthorizeRequest(id)
      if (info.consentRequired) {
        queryClient.removeQueries({ queryKey: oidcKeys.authorizeRequest(id) })
        await router.replace({ name: 'consent', query: { request_id: id } })
        return
      }
      const redirectTo = await completeAuthorize(id)
      window.location.assign(redirectTo)
      return
    }
    const redirect = route.query.redirect
    await router.replace(isLocalRedirect(redirect) ? redirect : { name: 'account' })
  } catch (error) {
    finishError.value = authorizeMessage(error)
  }
}

async function goToAccount() {
  await router.replace({ name: 'account' })
}
</script>

<template>
  <AuthShell
    :title="t('auth.enroll.title')"
    :description="t('auth.enroll.description')"
  >
    <AuthorizeRequestSummary
      v-if="authorizeQuery.data.value"
      :client-name="authorizeQuery.data.value.clientName"
      :scopes="authorizeQuery.data.value.scopes"
    />

    <Alert
      v-if="alertMessage"
      variant="destructive"
    >
      <AlertTitle>{{ t('errors.title') }}</AlertTitle>
      <AlertDescription>{{ alertMessage }}</AlertDescription>
    </Alert>

    <MfaEnrollmentFlow
      endpoint="/api/v1/auth/mfa/enroll"
      @complete="finish"
    />

    <Button
      v-if="alertMessage"
      type="button"
      variant="outline"
      class="w-full"
      @click="goToAccount"
    >
      {{ t('auth.enroll.goToAccount') }}
    </Button>
  </AuthShell>
</template>
