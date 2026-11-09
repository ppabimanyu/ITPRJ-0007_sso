<script setup lang="ts">
import { reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import type { AdminClient } from '@/lib/admin/types'

import {
  clientToForm,
  grantTypeOptions,
  parseTtl,
  splitLines,
  splitScopes,
  type ClientFormValues,
} from './client-form'

const props = defineProps<{
  initial: AdminClient | null
  pending: boolean
  fieldErrors: Readonly<Record<string, string>>
  submitLabel: string
  pendingLabel: string
  allowTypeChange: boolean
  showDisabled: boolean
}>()

const emit = defineEmits<{
  submit: [values: ClientFormValues]
  cancel: []
}>()

const { t } = useI18n()
const form = reactive(clientToForm(props.initial))

watch(
  () => props.initial,
  (initial) => {
    Object.assign(form, clientToForm(initial))
  },
)

function fieldError(name: string): string {
  return props.fieldErrors[name] ?? ''
}

function setType(value: unknown) {
  form.type = value as ClientFormValues['type']
}

function submit() {
  emit('submit', {
    name: form.name.trim(),
    type: form.type,
    redirectUris: splitLines(form.redirectUris),
    postLogoutRedirectUris: splitLines(form.postLogoutRedirectUris),
    backchannelLogoutUri:
      form.backchannelLogoutUri.trim() === '' ? null : form.backchannelLogoutUri.trim(),
    corsOrigins: splitLines(form.corsOrigins),
    grantTypes: [...form.grantTypes],
    allowedScopes: splitScopes(form.allowedScopes),
    thirdParty: form.thirdParty,
    accessTokenTtlSeconds: parseTtl(form.accessTokenTtl),
    idTokenTtlSeconds: parseTtl(form.idTokenTtl),
    disabled: form.disabled,
  })
}
</script>

<template>
  <form
    class="space-y-5"
    novalidate
    @submit.prevent="submit"
  >
    <div class="grid gap-4 sm:grid-cols-2">
      <div class="space-y-2">
        <Label for="client-name">{{ t('admin.clients.form.name') }}</Label>
        <Input
          id="client-name"
          v-model="form.name"
          :aria-invalid="fieldError('name') !== ''"
        />
        <p
          v-if="fieldError('name')"
          role="alert"
          class="text-sm text-destructive"
        >
          {{ fieldError('name') }}
        </p>
      </div>
      <div class="space-y-2">
        <Label for="client-type">{{ t('admin.clients.form.type') }}</Label>
        <Select
          :model-value="form.type"
          :disabled="!props.allowTypeChange"
          @update:model-value="setType"
        >
          <SelectTrigger
            id="client-type"
            class="w-full"
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="public">
              {{ t('admin.clients.type.public') }}
            </SelectItem>
            <SelectItem value="confidential">
              {{ t('admin.clients.type.confidential') }}
            </SelectItem>
          </SelectContent>
        </Select>
        <p class="text-xs text-muted-foreground">
          {{ t('admin.clients.form.typeHint') }}
        </p>
        <p
          v-if="fieldError('type')"
          role="alert"
          class="text-sm text-destructive"
        >
          {{ fieldError('type') }}
        </p>
      </div>
    </div>

    <div class="space-y-2">
      <Label for="client-redirect-uris">{{ t('admin.clients.form.redirectUris') }}</Label>
      <Textarea
        id="client-redirect-uris"
        v-model="form.redirectUris"
        :aria-invalid="fieldError('redirect_uris') !== ''"
      />
      <p class="text-xs text-muted-foreground">
        {{ t('admin.clients.form.redirectUrisHint') }}
      </p>
      <p
        v-if="fieldError('redirect_uris')"
        role="alert"
        class="text-sm text-destructive"
      >
        {{ fieldError('redirect_uris') }}
      </p>
    </div>

    <div class="grid gap-4 sm:grid-cols-2">
      <div class="space-y-2">
        <Label for="client-post-logout-uris">{{ t('admin.clients.form.postLogoutUris') }}</Label>
        <Textarea
          id="client-post-logout-uris"
          v-model="form.postLogoutRedirectUris"
          :aria-invalid="fieldError('post_logout_redirect_uris') !== ''"
        />
        <p
          v-if="fieldError('post_logout_redirect_uris')"
          role="alert"
          class="text-sm text-destructive"
        >
          {{ fieldError('post_logout_redirect_uris') }}
        </p>
      </div>
      <div class="space-y-2">
        <Label for="client-cors-origins">{{ t('admin.clients.form.corsOrigins') }}</Label>
        <Textarea
          id="client-cors-origins"
          v-model="form.corsOrigins"
          :aria-invalid="fieldError('cors_origins') !== ''"
        />
        <p class="text-xs text-muted-foreground">
          {{ t('admin.clients.form.corsOriginsHint') }}
        </p>
        <p
          v-if="fieldError('cors_origins')"
          role="alert"
          class="text-sm text-destructive"
        >
          {{ fieldError('cors_origins') }}
        </p>
      </div>
    </div>

    <div class="space-y-2">
      <Label for="client-backchannel">{{ t('admin.clients.form.backchannel') }}</Label>
      <Input
        id="client-backchannel"
        v-model="form.backchannelLogoutUri"
        :aria-invalid="fieldError('backchannel_logout_uri') !== ''"
      />
      <p
        v-if="fieldError('backchannel_logout_uri')"
        role="alert"
        class="text-sm text-destructive"
      >
        {{ fieldError('backchannel_logout_uri') }}
      </p>
    </div>

    <div class="space-y-2">
      <p class="text-sm font-medium">
        {{ t('admin.clients.form.grantTypes') }}
      </p>
      <div class="flex flex-wrap gap-4">
        <label
          v-for="grant in grantTypeOptions"
          :key="grant"
          class="flex items-center gap-2 text-sm"
        >
          <input
            v-model="form.grantTypes"
            type="checkbox"
            :value="grant"
            class="h-4 w-4 rounded border-input"
          >
          {{ t(`admin.clients.grant.${grant}`) }}
        </label>
      </div>
      <p
        v-if="fieldError('grant_types')"
        role="alert"
        class="text-sm text-destructive"
      >
        {{ fieldError('grant_types') }}
      </p>
    </div>

    <div class="space-y-2">
      <Label for="client-scopes">{{ t('admin.clients.form.scopes') }}</Label>
      <Textarea
        id="client-scopes"
        v-model="form.allowedScopes"
        :aria-invalid="fieldError('allowed_scopes') !== ''"
      />
      <p class="text-xs text-muted-foreground">
        {{ t('admin.clients.form.scopesHint') }}
      </p>
      <p
        v-if="fieldError('allowed_scopes')"
        role="alert"
        class="text-sm text-destructive"
      >
        {{ fieldError('allowed_scopes') }}
      </p>
    </div>

    <div class="grid gap-4 sm:grid-cols-2">
      <div class="space-y-2">
        <Label for="client-access-ttl">{{ t('admin.clients.form.accessTtl') }}</Label>
        <Input
          id="client-access-ttl"
          v-model="form.accessTokenTtl"
          inputmode="numeric"
          :aria-invalid="fieldError('access_token_ttl_seconds') !== ''"
        />
        <p class="text-xs text-muted-foreground">
          {{ t('admin.clients.form.ttlHint') }}
        </p>
        <p
          v-if="fieldError('access_token_ttl_seconds')"
          role="alert"
          class="text-sm text-destructive"
        >
          {{ fieldError('access_token_ttl_seconds') }}
        </p>
      </div>
      <div class="space-y-2">
        <Label for="client-id-ttl">{{ t('admin.clients.form.idTtl') }}</Label>
        <Input
          id="client-id-ttl"
          v-model="form.idTokenTtl"
          inputmode="numeric"
          :aria-invalid="fieldError('id_token_ttl_seconds') !== ''"
        />
        <p
          v-if="fieldError('id_token_ttl_seconds')"
          role="alert"
          class="text-sm text-destructive"
        >
          {{ fieldError('id_token_ttl_seconds') }}
        </p>
      </div>
    </div>

    <div class="flex items-center justify-between gap-4 rounded-lg border px-4 py-3">
      <div class="space-y-1">
        <Label for="client-third-party">{{ t('admin.clients.form.thirdParty') }}</Label>
        <p class="text-xs text-muted-foreground">
          {{ t('admin.clients.form.thirdPartyHint') }}
        </p>
      </div>
      <Switch
        id="client-third-party"
        v-model="form.thirdParty"
      />
    </div>

    <div
      v-if="props.showDisabled"
      class="flex items-center justify-between gap-4 rounded-lg border px-4 py-3"
    >
      <div class="space-y-1">
        <Label for="client-disabled">{{ t('admin.clients.form.disabled') }}</Label>
        <p class="text-xs text-muted-foreground">
          {{ t('admin.clients.form.disabledHint') }}
        </p>
      </div>
      <Switch
        id="client-disabled"
        v-model="form.disabled"
      />
    </div>

    <div class="flex flex-wrap justify-end gap-2">
      <Button
        type="button"
        variant="outline"
        :disabled="props.pending"
        @click="emit('cancel')"
      >
        {{ t('admin.common.cancel') }}
      </Button>
      <Button
        type="submit"
        :disabled="props.pending"
      >
        {{ props.pending ? props.pendingLabel : props.submitLabel }}
      </Button>
    </div>
  </form>
</template>
