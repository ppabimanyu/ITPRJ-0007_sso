<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import type { MfaEnrollmentEndpoint } from '@/lib/auth-api'

import { useDisableTotp, useRegenerateRecoveryCodes } from '../auth/queries'
import { useAuthError } from '../auth/use-auth-error'
import MfaEnrollmentFlow from '../mfa/MfaEnrollmentFlow.vue'
import RecoveryCodeList from '../mfa/RecoveryCodeList.vue'

const props = defineProps<{
  enrolled: boolean
  isAdmin: boolean
}>()

const { t } = useI18n()
const { generalMessage } = useAuthError()
const {
  mutateAsync: disable,
  isPending: disabling,
  error: disableError,
  reset: resetDisable,
} = useDisableTotp()
const {
  mutateAsync: regenerate,
  isPending: regenerating,
  error: regenerateError,
  reset: resetRegenerate,
} = useRegenerateRecoveryCodes()

const showSetup = ref(false)
const recoveryCodes = ref<string[]>([])
const dialog = ref<HTMLDialogElement | null>(null)

// An admin without MFA is blocked from /api/v1/me by design, so setup must use
// the auth enrollment route that stays open during forced enrollment.
const setupEndpoint = computed<MfaEnrollmentEndpoint>(() =>
  props.isAdmin && !props.enrolled ? '/api/v1/auth/mfa/enroll' : '/api/v1/me/mfa/totp',
)

const disableMessage = computed(() =>
  disableError.value === null
    ? ''
    : generalMessage(disableError.value, { badRequest: 'account.mfa.notEnrolled' }),
)
const regenerateMessage = computed(() =>
  regenerateError.value === null
    ? ''
    : generalMessage(regenerateError.value, { badRequest: 'account.mfa.notEnrolled' }),
)

function beginSetup() {
  resetRegenerate()
  recoveryCodes.value = []
  showSetup.value = true
}

function finishSetup() {
  showSetup.value = false
}

async function requestCodes() {
  try {
    recoveryCodes.value = await regenerate()
  } catch {
    // The mutation error renders above the actions.
  }
}

function dismissCodes() {
  recoveryCodes.value = []
}

function openDisable() {
  resetDisable()
  dialog.value?.showModal()
}

function closeDisable() {
  dialog.value?.close()
}

async function confirmDisable() {
  try {
    await disable()
    dialog.value?.close()
    recoveryCodes.value = []
  } catch {
    // The mutation error renders inside the dialog.
  }
}
</script>

<template>
  <section class="space-y-4">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div class="space-y-1">
        <h2 class="text-lg font-semibold tracking-tight">
          {{ t('account.mfa.title') }}
        </h2>
        <p class="text-sm text-muted-foreground">
          {{ t('account.mfa.description') }}
        </p>
      </div>
      <Badge :variant="props.enrolled ? 'default' : 'secondary'">
        {{ props.enrolled ? t('account.mfa.enabled') : t('account.mfa.disabled') }}
      </Badge>
    </div>

    <Alert
      v-if="regenerateMessage && recoveryCodes.length === 0"
      variant="destructive"
    >
      <AlertTitle>{{ t('errors.title') }}</AlertTitle>
      <AlertDescription>{{ regenerateMessage }}</AlertDescription>
    </Alert>

    <MfaEnrollmentFlow
      v-if="showSetup"
      :endpoint="setupEndpoint"
      @complete="finishSetup"
    />

    <RecoveryCodeList
      v-else-if="recoveryCodes.length > 0"
      :codes="recoveryCodes"
      :title="t('account.mfa.recoveryTitle')"
      :description="t('account.mfa.recoveryDescription')"
      :warning="t('account.mfa.recoveryWarning')"
    >
      <template #actions>
        <Button
          type="button"
          @click="dismissCodes"
        >
          {{ t('account.mfa.recoveryDone') }}
        </Button>
      </template>
    </RecoveryCodeList>

    <template v-else-if="props.enrolled">
      <p class="text-sm text-muted-foreground">
        {{ t('account.mfa.enabledHint') }}
      </p>
      <div class="flex flex-wrap items-center gap-3">
        <Button
          type="button"
          variant="outline"
          :disabled="regenerating"
          @click="requestCodes"
        >
          {{ regenerating ? t('account.mfa.regenerating') : t('account.mfa.regenerate') }}
        </Button>
        <Button
          type="button"
          variant="destructive"
          :disabled="disabling"
          @click="openDisable"
        >
          {{ t('account.mfa.disable') }}
        </Button>
      </div>
    </template>

    <template v-else>
      <p class="text-sm text-muted-foreground">
        {{ t('account.mfa.disabledHint') }}
      </p>
      <Button
        type="button"
        @click="beginSetup"
      >
        {{ t('account.mfa.enable') }}
      </Button>
    </template>

    <dialog
      ref="dialog"
      class="w-full max-w-md rounded-xl border bg-card p-6 text-card-foreground shadow-lg backdrop:bg-black/40"
      @close="resetDisable"
    >
      <div class="space-y-4">
        <div class="space-y-2">
          <h3 class="text-lg font-semibold tracking-tight">
            {{ t('account.mfa.disableConfirmTitle') }}
          </h3>
          <p class="text-sm text-muted-foreground">
            {{ t('account.mfa.disableConfirmDescription') }}
          </p>
        </div>

        <Alert
          v-if="disableMessage"
          variant="destructive"
        >
          <AlertTitle>{{ t('errors.title') }}</AlertTitle>
          <AlertDescription>{{ disableMessage }}</AlertDescription>
        </Alert>

        <div class="flex flex-wrap justify-end gap-3">
          <Button
            type="button"
            variant="outline"
            :disabled="disabling"
            @click="closeDisable"
          >
            {{ t('account.mfa.cancel') }}
          </Button>
          <Button
            type="button"
            variant="destructive"
            :disabled="disabling"
            @click="confirmDisable"
          >
            {{ disabling ? t('account.mfa.disabling') : t('account.mfa.disableConfirm') }}
          </Button>
        </div>
      </div>
    </dialog>
  </section>
</template>
