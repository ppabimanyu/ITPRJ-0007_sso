<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuery } from '@tanstack/vue-query'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'

import { getRealmSettings } from '@/lib/admin/api'
import { adminKeys } from '@/lib/query-keys'

import AdminProblem from '../components/AdminProblem.vue'
import { useUpdateRealmSettings } from '../queries'
import { useAdminError } from '../use-admin-error'

const { t } = useI18n()
const { fieldErrors } = useAdminError()

const settingsQuery = useQuery({
  queryKey: adminKeys.settings,
  queryFn: getRealmSettings,
})

const updateMutation = useUpdateRealmSettings()
const saved = ref(false)

const form = reactive({
  minLength: '12',
  idleMinutes: '30',
  absoluteHours: '12',
  adminIdleMinutes: '15',
  rememberMeDays: '7',
  smtpHost: '',
  smtpPort: '587',
  smtpUsername: '',
  smtpFrom: '',
  smtpUseTls: false,
  smtpPassword: '',
})

const settings = computed(() => settingsQuery.data.value ?? null)
const fields = computed(() => fieldErrors(updateMutation.error.value))

watch(
  () => settings.value,
  (value) => {
    if (!value) {
      return
    }
    form.minLength = String(value.passwordPolicy.minLength)
    form.idleMinutes = String(value.sessionPolicy.idleMinutes)
    form.absoluteHours = String(value.sessionPolicy.absoluteHours)
    form.adminIdleMinutes = String(value.sessionPolicy.adminIdleMinutes)
    form.rememberMeDays = String(value.sessionPolicy.rememberMeDays)
    form.smtpHost = value.smtp.host
    form.smtpPort = String(value.smtp.port)
    form.smtpUsername = value.smtp.username ?? ''
    form.smtpFrom = value.smtp.fromAddress
    form.smtpUseTls = value.smtp.useTls
    form.smtpPassword = ''
  },
  { immediate: true },
)

function fieldError(name: string): string {
  return fields.value[name] ?? ''
}

function numberValue(raw: string): number {
  const parsed = Number(raw.trim())
  return Number.isFinite(parsed) ? parsed : 0
}

async function save() {
  saved.value = false
  try {
    await updateMutation.mutateAsync({
      passwordPolicy: { minLength: numberValue(form.minLength) },
      sessionPolicy: {
        idleMinutes: numberValue(form.idleMinutes),
        absoluteHours: numberValue(form.absoluteHours),
        adminIdleMinutes: numberValue(form.adminIdleMinutes),
        rememberMeDays: numberValue(form.rememberMeDays),
      },
      smtp: {
        host: form.smtpHost.trim(),
        port: numberValue(form.smtpPort),
        username: form.smtpUsername.trim() === '' ? null : form.smtpUsername.trim(),
        fromAddress: form.smtpFrom.trim(),
        useTls: form.smtpUseTls,
        ...(form.smtpPassword === '' ? {} : { password: form.smtpPassword }),
      },
    })
    form.smtpPassword = ''
    saved.value = true
  } catch {
    // The mutation error renders above the form.
  }
}
</script>

<template>
  <div class="space-y-6">
    <div class="space-y-1">
      <h2 class="text-lg font-semibold tracking-tight">
        {{ t('admin.settings.title') }}
      </h2>
      <p class="text-sm text-muted-foreground">
        {{ t('admin.settings.description') }}
      </p>
    </div>

    <AdminProblem :error="settingsQuery.error.value" />

    <p
      v-if="settingsQuery.isPending.value"
      class="text-sm text-muted-foreground"
    >
      {{ t('common.loading') }}
    </p>

    <form
      v-else-if="settings"
      class="space-y-6"
      novalidate
      @submit.prevent="save"
    >
      <Alert
        v-if="saved"
      >
        <AlertDescription>{{ t('admin.settings.saved') }}</AlertDescription>
      </Alert>
      <AdminProblem :error="updateMutation.error.value" />

      <Card>
        <CardHeader>
          <CardTitle>{{ t('admin.settings.passwordPolicy.title') }}</CardTitle>
          <CardDescription>{{ t('admin.settings.passwordPolicy.description') }}</CardDescription>
        </CardHeader>
        <CardContent class="space-y-4">
          <div class="max-w-xs space-y-2">
            <Label for="settings-min-length">{{ t('admin.settings.passwordPolicy.minLength') }}</Label>
            <Input
              id="settings-min-length"
              v-model="form.minLength"
              inputmode="numeric"
              :aria-invalid="fieldError('password_policy.min_length') !== ''"
            />
            <p class="text-xs text-muted-foreground">
              {{ t('admin.settings.passwordPolicy.minLengthHint') }}
            </p>
            <p
              v-if="fieldError('password_policy.min_length')"
              role="alert"
              class="text-sm text-destructive"
            >
              {{ fieldError('password_policy.min_length') }}
            </p>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{{ t('admin.settings.sessionPolicy.title') }}</CardTitle>
          <CardDescription>{{ t('admin.settings.sessionPolicy.description') }}</CardDescription>
        </CardHeader>
        <CardContent>
          <div class="grid gap-4 sm:grid-cols-2">
            <div class="space-y-2">
              <Label for="settings-idle-minutes">{{ t('admin.settings.sessionPolicy.idleMinutes') }}</Label>
              <Input
                id="settings-idle-minutes"
                v-model="form.idleMinutes"
                inputmode="numeric"
                :aria-invalid="fieldError('session_policy.idle_minutes') !== ''"
              />
              <p
                v-if="fieldError('session_policy.idle_minutes')"
                role="alert"
                class="text-sm text-destructive"
              >
                {{ fieldError('session_policy.idle_minutes') }}
              </p>
            </div>
            <div class="space-y-2">
              <Label for="settings-absolute-hours">{{ t('admin.settings.sessionPolicy.absoluteHours') }}</Label>
              <Input
                id="settings-absolute-hours"
                v-model="form.absoluteHours"
                inputmode="numeric"
                :aria-invalid="fieldError('session_policy.absolute_hours') !== ''"
              />
              <p
                v-if="fieldError('session_policy.absolute_hours')"
                role="alert"
                class="text-sm text-destructive"
              >
                {{ fieldError('session_policy.absolute_hours') }}
              </p>
            </div>
            <div class="space-y-2">
              <Label for="settings-admin-idle">{{ t('admin.settings.sessionPolicy.adminIdleMinutes') }}</Label>
              <Input
                id="settings-admin-idle"
                v-model="form.adminIdleMinutes"
                inputmode="numeric"
                :aria-invalid="fieldError('session_policy.admin_idle_minutes') !== ''"
              />
              <p
                v-if="fieldError('session_policy.admin_idle_minutes')"
                role="alert"
                class="text-sm text-destructive"
              >
                {{ fieldError('session_policy.admin_idle_minutes') }}
              </p>
            </div>
            <div class="space-y-2">
              <Label for="settings-remember-days">{{ t('admin.settings.sessionPolicy.rememberMeDays') }}</Label>
              <Input
                id="settings-remember-days"
                v-model="form.rememberMeDays"
                inputmode="numeric"
                :aria-invalid="fieldError('session_policy.remember_me_days') !== ''"
              />
              <p
                v-if="fieldError('session_policy.remember_me_days')"
                role="alert"
                class="text-sm text-destructive"
              >
                {{ fieldError('session_policy.remember_me_days') }}
              </p>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{{ t('admin.settings.smtp.title') }}</CardTitle>
          <CardDescription>{{ t('admin.settings.smtp.description') }}</CardDescription>
        </CardHeader>
        <CardContent class="space-y-4">
          <div class="grid gap-4 sm:grid-cols-2">
            <div class="space-y-2">
              <Label for="settings-smtp-host">{{ t('admin.settings.smtp.host') }}</Label>
              <Input
                id="settings-smtp-host"
                v-model="form.smtpHost"
                :aria-invalid="fieldError('smtp.host') !== ''"
              />
              <p
                v-if="fieldError('smtp.host')"
                role="alert"
                class="text-sm text-destructive"
              >
                {{ fieldError('smtp.host') }}
              </p>
            </div>
            <div class="space-y-2">
              <Label for="settings-smtp-port">{{ t('admin.settings.smtp.port') }}</Label>
              <Input
                id="settings-smtp-port"
                v-model="form.smtpPort"
                inputmode="numeric"
                :aria-invalid="fieldError('smtp.port') !== ''"
              />
              <p
                v-if="fieldError('smtp.port')"
                role="alert"
                class="text-sm text-destructive"
              >
                {{ fieldError('smtp.port') }}
              </p>
            </div>
            <div class="space-y-2">
              <Label for="settings-smtp-username">{{ t('admin.settings.smtp.username') }}</Label>
              <Input
                id="settings-smtp-username"
                v-model="form.smtpUsername"
                autocomplete="off"
                :aria-invalid="fieldError('smtp.username') !== ''"
              />
              <p
                v-if="fieldError('smtp.username')"
                role="alert"
                class="text-sm text-destructive"
              >
                {{ fieldError('smtp.username') }}
              </p>
            </div>
            <div class="space-y-2">
              <Label for="settings-smtp-from">{{ t('admin.settings.smtp.fromAddress') }}</Label>
              <Input
                id="settings-smtp-from"
                v-model="form.smtpFrom"
                type="email"
                :aria-invalid="fieldError('smtp.from_address') !== ''"
              />
              <p
                v-if="fieldError('smtp.from_address')"
                role="alert"
                class="text-sm text-destructive"
              >
                {{ fieldError('smtp.from_address') }}
              </p>
            </div>
          </div>

          <div class="flex items-center justify-between gap-4 rounded-lg border px-4 py-3">
            <div class="space-y-1">
              <Label for="settings-smtp-tls">{{ t('admin.settings.smtp.useTls') }}</Label>
              <p class="text-xs text-muted-foreground">
                {{ t('admin.settings.smtp.useTlsHint') }}
              </p>
            </div>
            <Switch
              id="settings-smtp-tls"
              v-model="form.smtpUseTls"
            />
          </div>

          <div class="space-y-2">
            <div class="flex flex-wrap items-center gap-2">
              <Label for="settings-smtp-password">{{ t('admin.settings.smtp.password') }}</Label>
              <Badge :variant="settings.smtp.passwordSet ? 'secondary' : 'outline'">
                {{ settings.smtp.passwordSet ? t('admin.settings.smtp.passwordSet') : t('admin.settings.smtp.passwordNotSet') }}
              </Badge>
            </div>
            <Input
              id="settings-smtp-password"
              v-model="form.smtpPassword"
              type="password"
              autocomplete="new-password"
              :aria-invalid="fieldError('smtp.password') !== ''"
            />
            <p class="text-xs text-muted-foreground">
              {{ t('admin.settings.smtp.passwordHint') }}
            </p>
            <p
              v-if="fieldError('smtp.password')"
              role="alert"
              class="text-sm text-destructive"
            >
              {{ fieldError('smtp.password') }}
            </p>
          </div>
        </CardContent>
      </Card>

      <Button
        type="submit"
        :disabled="updateMutation.isPending.value"
      >
        {{ updateMutation.isPending.value ? t('admin.common.saving') : t('admin.settings.save') }}
      </Button>
    </form>
  </div>
</template>
