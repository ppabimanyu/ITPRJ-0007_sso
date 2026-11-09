<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import type { User } from '@/lib/auth-api'

import { useUpdateProfile } from '../auth/queries'
import { useAuthError } from '../auth/use-auth-error'

const props = defineProps<{
  user: User
}>()

const { t } = useI18n()
const { mutateAsync: saveProfile, isPending, error } = useUpdateProfile()
const { generalMessage, fieldErrors } = useAuthError()

const form = reactive({
  name: '',
  givenName: '',
  familyName: '',
  locale: 'en' as 'en' | 'id',
})
const saved = ref(false)

watch(
  () => props.user,
  (user) => {
    form.name = user.name ?? ''
    form.givenName = user.givenName ?? ''
    form.familyName = user.familyName ?? ''
    form.locale = user.locale === 'id' ? 'id' : 'en'
  },
  { immediate: true },
)

const errorMessage = computed(() => (error.value === null ? '' : generalMessage(error.value)))
const fields = computed(() => fieldErrors(error.value))

function fieldError(name: string) {
  return fields.value[name] ?? ''
}

async function submit() {
  saved.value = false
  try {
    await saveProfile({
      name: form.name.trim(),
      givenName: form.givenName.trim(),
      familyName: form.familyName.trim(),
      locale: form.locale,
    })
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
        {{ t('account.profile.title') }}
      </h2>
      <p class="text-sm text-muted-foreground">
        {{ t('account.profile.description') }}
      </p>
    </div>

    <Alert
      v-if="saved"
      variant="success"
    >
      <AlertDescription>{{ t('account.profile.saved') }}</AlertDescription>
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
        <Label for="profile-email">{{ t('account.profile.email') }}</Label>
        <p
          id="profile-email"
          class="text-sm text-muted-foreground"
        >
          {{ user.email }}
        </p>
        <p class="text-xs text-muted-foreground">
          {{ t('account.profile.emailHint') }}
        </p>
      </div>

      <div class="space-y-2">
        <Label for="profile-name">{{ t('account.profile.name') }}</Label>
        <Input
          id="profile-name"
          v-model="form.name"
          autocomplete="name"
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

      <div class="grid gap-4 sm:grid-cols-2">
        <div class="space-y-2">
          <Label for="profile-given-name">{{ t('account.profile.givenName') }}</Label>
          <Input
            id="profile-given-name"
            v-model="form.givenName"
            autocomplete="given-name"
            :aria-invalid="fieldError('given_name') !== ''"
          />
          <p
            v-if="fieldError('given_name')"
            role="alert"
            class="text-sm text-destructive"
          >
            {{ fieldError('given_name') }}
          </p>
        </div>
        <div class="space-y-2">
          <Label for="profile-family-name">{{ t('account.profile.familyName') }}</Label>
          <Input
            id="profile-family-name"
            v-model="form.familyName"
            autocomplete="family-name"
            :aria-invalid="fieldError('family_name') !== ''"
          />
          <p
            v-if="fieldError('family_name')"
            role="alert"
            class="text-sm text-destructive"
          >
            {{ fieldError('family_name') }}
          </p>
        </div>
      </div>

      <div class="space-y-2">
        <Label for="profile-locale">{{ t('account.profile.locale') }}</Label>
        <select
          id="profile-locale"
          v-model="form.locale"
          class="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background"
        >
          <option value="en">
            {{ t('account.profile.localeEn') }}
          </option>
          <option value="id">
            {{ t('account.profile.localeId') }}
          </option>
        </select>
        <p
          v-if="fieldError('locale')"
          role="alert"
          class="text-sm text-destructive"
        >
          {{ fieldError('locale') }}
        </p>
      </div>

      <Button
        type="submit"
        :disabled="isPending"
      >
        {{ isPending ? t('account.profile.submitting') : t('account.profile.submit') }}
      </Button>
    </form>
  </section>
</template>
