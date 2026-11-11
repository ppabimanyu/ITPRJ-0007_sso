<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'

import type { AdminUser } from '@/lib/admin/types'

import { useCreateUser } from '../queries'
import { useAdminError } from '../use-admin-error'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  created: [user: AdminUser]
}>()

const { t } = useI18n()
const { mutateAsync: create, isPending, error: requestError, reset } = useCreateUser()
const { describe, fieldErrors } = useAdminError()

const form = reactive({ email: '', name: '', username: '', sendInvite: true })
const created = ref<AdminUser | null>(null)
const idempotencyKey = ref('')

watch(
  () => props.open,
  (open) => {
    if (!open) {
      return
    }
    form.email = ''
    form.name = ''
    form.username = ''
    form.sendInvite = true
    created.value = null
    idempotencyKey.value = ''
    reset()
  },
)

function fieldError(name: string): string {
  return fieldErrors(requestError.value)[name] ?? ''
}

async function submit() {
  if (isPending.value) {
    return
  }
  created.value = null
  const key = crypto.randomUUID()
  idempotencyKey.value = key
  try {
    const user = await create({
      email: form.email.trim(),
      name: form.name.trim(),
      username: form.username.trim() === '' ? null : form.username.trim(),
      sendInvite: form.sendInvite,
      idempotencyKey: key,
    })
    created.value = user
    emit('created', user)
  } catch {
    // The request error renders above the form.
  }
}
</script>

<template>
  <Dialog
    :open="props.open"
    @update:open="emit('update:open', $event)"
  >
    <DialogContent class="max-h-[90vh] overflow-y-auto">
      <DialogHeader>
        <DialogTitle>{{ t('admin.users.create.title') }}</DialogTitle>
        <DialogDescription>{{ t('admin.users.create.description') }}</DialogDescription>
      </DialogHeader>
      <div
        v-if="created"
        class="space-y-4"
      >
        <Alert>
          <AlertTitle>{{ t('admin.users.create.createdTitle') }}</AlertTitle>
          <AlertDescription>
            {{ t('admin.users.create.createdDescription', { email: created.email }) }}
          </AlertDescription>
        </Alert>
        <div class="space-y-1">
          <p class="text-sm font-medium">
            {{ t('admin.users.create.idempotencyKey') }}
          </p>
          <code class="block select-all break-all rounded-md border bg-muted/40 px-3 py-2 font-mono text-xs">
            {{ idempotencyKey }}
          </code>
          <p class="text-xs text-muted-foreground">
            {{ t('admin.users.create.idempotencyHint') }}
          </p>
        </div>
        <div class="flex justify-end">
          <Button
            type="button"
            @click="emit('update:open', false)"
          >
            {{ t('admin.common.done') }}
          </Button>
        </div>
      </div>

      <form
        v-else
        class="space-y-4"
        novalidate
        @submit.prevent="submit"
      >
        <Alert
          v-if="requestError"
          variant="destructive"
        >
          <AlertTitle>{{ t('admin.errors.title') }}</AlertTitle>
          <AlertDescription>
            {{ describe(requestError).message }}
          </AlertDescription>
        </Alert>

        <div class="space-y-2">
          <Label for="create-user-email">{{ t('admin.users.create.email') }}</Label>
          <Input
            id="create-user-email"
            v-model="form.email"
            type="email"
            autocomplete="off"
            :aria-invalid="fieldError('email') !== ''"
          />
          <p
            v-if="fieldError('email')"
            role="alert"
            class="text-sm text-destructive"
          >
            {{ fieldError('email') }}
          </p>
        </div>

        <div class="space-y-2">
          <Label for="create-user-name">{{ t('admin.users.create.name') }}</Label>
          <Input
            id="create-user-name"
            v-model="form.name"
            autocomplete="off"
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
          <Label for="create-user-username">{{ t('admin.users.create.username') }}</Label>
          <Input
            id="create-user-username"
            v-model="form.username"
            autocomplete="off"
            :aria-invalid="fieldError('username') !== ''"
          />
          <p class="text-xs text-muted-foreground">
            {{ t('admin.users.create.usernameHint') }}
          </p>
          <p
            v-if="fieldError('username')"
            role="alert"
            class="text-sm text-destructive"
          >
            {{ fieldError('username') }}
          </p>
        </div>

        <div class="flex items-center justify-between gap-4 rounded-lg border px-4 py-3">
          <div class="space-y-1">
            <Label for="create-user-invite">{{ t('admin.users.create.sendInvite') }}</Label>
            <p class="text-xs text-muted-foreground">
              {{ t('admin.users.create.sendInviteHint') }}
            </p>
          </div>
          <Switch
            id="create-user-invite"
            v-model="form.sendInvite"
          />
        </div>

        <div class="flex flex-wrap justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            :disabled="isPending"
            @click="emit('update:open', false)"
          >
            {{ t('admin.common.cancel') }}
          </Button>
          <Button
            type="submit"
            :disabled="isPending"
          >
            {{ isPending ? t('admin.users.create.submitting') : t('admin.users.create.submit') }}
          </Button>
        </div>

        <p
          v-if="idempotencyKey"
          class="text-xs text-muted-foreground"
        >
          {{ t('admin.users.create.idempotencyKey') }}:
          <code class="select-all break-all font-mono">{{ idempotencyKey }}</code>
        </p>
      </form>
    </DialogContent>
  </Dialog>
</template>
