<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { QrcodeSvg } from 'qrcode.vue'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { isApiError } from '@/lib/http'
import type { MfaEnrollmentEndpoint } from '@/lib/auth-api'

import {
  useConfirmMfaEnrollment,
  useStartMfaEnrollment,
} from '../auth/queries'
import { useAuthError } from '../auth/use-auth-error'
import RecoveryCodeList from './RecoveryCodeList.vue'

type Phase = 'starting' | 'confirm' | 'codes' | 'already' | 'error'

const props = defineProps<{
  endpoint: MfaEnrollmentEndpoint
}>()

const emit = defineEmits<{
  complete: []
}>()

const { t } = useI18n()
const { mutateAsync: startEnrollment, isPending: starting } = useStartMfaEnrollment()
const { mutateAsync: confirmEnrollment, isPending: confirming } = useConfirmMfaEnrollment()
const { generalMessage } = useAuthError()

const phase = ref<Phase>('starting')
const secret = ref('')
const otpauthUri = ref('')
const recoveryCodes = ref<string[]>([])
const startError = ref<unknown>(null)
const confirmError = ref<unknown>(null)
const copied = ref<'secret' | 'uri' | null>(null)
const copyFailed = ref(false)
const codeInput = ref<{ focus: () => void } | null>(null)
const form = reactive({ code: '' })

const code = computed({
  get: () => form.code,
  set: (value: string) => {
    form.code = value.replace(/\D/g, '').slice(0, 6)
  },
})

const startMessage = computed(() =>
  startError.value === null
    ? ''
    : generalMessage(startError.value, { badRequest: 'auth.enroll.startFailed' }),
)
const confirmMessage = computed(() => {
  if (confirmError.value === null) {
    return ''
  }
  if (isApiError(confirmError.value) && confirmError.value.status === 400) {
    return t('auth.enroll.invalid')
  }
  return generalMessage(confirmError.value, { badRequest: 'auth.enroll.invalid' })
})

onMounted(begin)

async function begin() {
  phase.value = 'starting'
  startError.value = null
  try {
    const result = await startEnrollment(props.endpoint)
    secret.value = result.secret
    otpauthUri.value = result.otpauthUri
    phase.value = 'confirm'
    await nextTick()
    codeInput.value?.focus()
  } catch (error) {
    if (isApiError(error) && error.status === 409) {
      phase.value = 'already'
      return
    }
    startError.value = error
    phase.value = 'error'
  }
}

async function submit() {
  confirmError.value = null
  try {
    const result = await confirmEnrollment({ endpoint: props.endpoint, code: form.code })
    recoveryCodes.value = result.recoveryCodes
    form.code = ''
    phase.value = 'codes'
  } catch (error) {
    if (isApiError(error) && error.status === 409) {
      phase.value = 'already'
      return
    }
    confirmError.value = error
    await nextTick()
    codeInput.value?.focus()
  }
}

async function copy(value: string, field: 'secret' | 'uri') {
  copyFailed.value = false
  try {
    await navigator.clipboard.writeText(value)
    copied.value = field
  } catch {
    copied.value = null
    copyFailed.value = true
  }
}

function complete() {
  emit('complete')
}
</script>

<template>
  <div class="space-y-4">
    <p
      v-if="phase === 'starting'"
      class="text-sm text-muted-foreground"
    >
      {{ t('auth.enroll.starting') }}
    </p>

    <template v-else-if="phase === 'error'">
      <Alert variant="destructive">
        <AlertTitle>{{ t('errors.title') }}</AlertTitle>
        <AlertDescription>{{ startMessage }}</AlertDescription>
      </Alert>
      <Button
        variant="outline"
        :disabled="starting"
        @click="begin"
      >
        {{ t('auth.enroll.retry') }}
      </Button>
    </template>

    <template v-else-if="phase === 'already'">
      <Alert>
        <AlertTitle>{{ t('auth.enroll.alreadyTitle') }}</AlertTitle>
        <AlertDescription>{{ t('auth.enroll.alreadyDescription') }}</AlertDescription>
      </Alert>
      <Button @click="complete">
        {{ t('auth.enroll.continue') }}
      </Button>
    </template>

    <form
      v-else-if="phase === 'confirm'"
      class="space-y-4"
      novalidate
      @submit.prevent="submit"
    >
      <div class="flex flex-col items-center gap-3">
        <div
          class="rounded-md border border-brass/40 bg-white p-2"
          role="img"
          :aria-label="t('auth.enroll.qrAlt')"
        >
          <QrcodeSvg
            v-if="otpauthUri"
            :value="otpauthUri"
            :size="176"
            level="M"
            :margin="4"
            background="#ffffff"
            foreground="#101a15"
            aria-hidden="true"
          />
        </div>
        <div class="space-y-1 text-center">
          <p class="font-display text-lg leading-snug">
            {{ t('auth.enroll.qrLabel') }}
          </p>
          <p class="max-w-sm text-sm text-muted-foreground">
            {{ t('auth.enroll.qrHint') }}
          </p>
        </div>
      </div>

      <div class="flex items-center gap-3 pt-1">
        <span class="text-xs text-muted-foreground">{{ t('auth.enroll.manualLabel') }}</span>
        <span
          class="h-px flex-1 bg-border"
          aria-hidden="true"
        />
      </div>

      <div class="space-y-2">
        <Label for="mfa-enroll-secret">{{ t('auth.enroll.secretLabel') }}</Label>
        <div class="flex items-start gap-2">
          <code
            id="mfa-enroll-secret"
            class="min-w-0 flex-1 select-all break-all rounded-md border bg-muted/40 px-3 py-2 font-mono text-sm"
          >
            {{ secret }}
          </code>
          <Button
            type="button"
            variant="outline"
            size="sm"
            @click="copy(secret, 'secret')"
          >
            {{ copied === 'secret' ? t('auth.enroll.copied') : t('auth.enroll.copy') }}
          </Button>
        </div>
        <p class="text-xs text-muted-foreground">
          {{ t('auth.enroll.secretHint') }}
        </p>
      </div>

      <div class="space-y-2">
        <Label for="mfa-enroll-uri">{{ t('auth.enroll.uriLabel') }}</Label>
        <div class="flex items-start gap-2">
          <code
            id="mfa-enroll-uri"
            class="min-w-0 flex-1 select-all break-all rounded-md border bg-muted/40 px-3 py-2 font-mono text-xs"
          >
            {{ otpauthUri }}
          </code>
          <Button
            type="button"
            variant="outline"
            size="sm"
            @click="copy(otpauthUri, 'uri')"
          >
            {{ copied === 'uri' ? t('auth.enroll.copied') : t('auth.enroll.copy') }}
          </Button>
        </div>
        <p class="text-xs text-muted-foreground">
          {{ t('auth.enroll.uriHint') }}
        </p>
      </div>

      <Alert
        v-if="copyFailed"
        variant="destructive"
      >
        <AlertDescription>{{ t('auth.enroll.copyFailed') }}</AlertDescription>
      </Alert>

      <div class="space-y-2">
        <Label for="mfa-enroll-code">{{ t('auth.enroll.codeLabel') }}</Label>
        <Input
          id="mfa-enroll-code"
          ref="codeInput"
          v-model="code"
          inputmode="numeric"
          autocomplete="one-time-code"
          maxlength="6"
          pattern="[0-9]*"
          required
          :aria-invalid="confirmMessage !== ''"
          :aria-describedby="confirmMessage === '' ? undefined : 'mfa-enroll-code-error'"
        />
        <p
          v-if="confirmMessage"
          id="mfa-enroll-code-error"
          role="alert"
          class="text-sm text-destructive"
        >
          {{ confirmMessage }}
        </p>
      </div>

      <Button
        type="submit"
        :disabled="confirming || form.code.length !== 6"
      >
        {{ confirming ? t('auth.enroll.confirming') : t('auth.enroll.confirm') }}
      </Button>
    </form>

    <template v-else-if="phase === 'codes'">
      <RecoveryCodeList
        :codes="recoveryCodes"
        :title="t('auth.enroll.recoveryTitle')"
        :description="t('auth.enroll.recoveryDescription')"
        :warning="t('auth.enroll.recoveryWarning')"
      >
        <template #actions>
          <Button
            type="button"
            @click="complete"
          >
            {{ t('auth.enroll.saved') }}
          </Button>
        </template>
      </RecoveryCodeList>
    </template>
  </div>
</template>
