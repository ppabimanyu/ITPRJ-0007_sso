<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

import { useRevokeSession, useSessions } from '../auth/queries'
import { useAuthError } from '../auth/use-auth-error'

const { t, locale } = useI18n()
const { data, isPending, error } = useSessions()
const {
  mutateAsync: revoke,
  isPending: revokePending,
  variables,
  error: revokeFailure,
} = useRevokeSession()
const { generalMessage } = useAuthError()

const sessions = computed(() => data.value ?? [])
const currentSession = computed(() => sessions.value.find((session) => session.current) ?? null)
const otherSessions = computed(() => sessions.value.filter((session) => !session.current))
const listingError = computed(() => (error.value === null ? '' : generalMessage(error.value)))
const revokeError = computed(() =>
  revokeFailure.value === null ? '' : generalMessage(revokeFailure.value),
)

const dateFormat = computed(
  () => new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }),
)

function formatDate(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : dateFormat.value.format(date)
}

function deviceLabel(userAgent: string | null): string {
  return userAgent === null || userAgent === ''
    ? t('account.sessions.unknownDevice')
    : userAgent
}

async function revokeSession(id: string) {
  try {
    await revoke(id)
  } catch {
    // The mutation error renders above the list.
  }
}
</script>

<template>
  <section class="space-y-4">
    <div class="space-y-1">
      <h2 class="text-lg font-semibold tracking-tight">
        {{ t('account.sessions.title') }}
      </h2>
      <p class="text-sm text-muted-foreground">
        {{ t('account.sessions.description') }}
      </p>
    </div>

    <Alert
      v-if="listingError"
      variant="destructive"
    >
      <AlertTitle>{{ t('errors.title') }}</AlertTitle>
      <AlertDescription>{{ listingError }}</AlertDescription>
    </Alert>
    <Alert
      v-if="revokeError"
      variant="destructive"
    >
      <AlertTitle>{{ t('errors.title') }}</AlertTitle>
      <AlertDescription>{{ revokeError }}</AlertDescription>
    </Alert>

    <p
      v-if="isPending"
      class="text-sm text-muted-foreground"
    >
      {{ t('common.loading') }}
    </p>

    <template v-else>
      <div
        v-if="currentSession"
        class="rounded-lg border bg-muted/40 p-4"
      >
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div class="space-y-1">
            <p class="text-sm font-medium">
              {{ t('account.sessions.currentTitle') }}
            </p>
            <p class="text-xs text-muted-foreground">
              {{ t('account.sessions.currentDescription') }}
            </p>
          </div>
          <Badge variant="secondary">
            {{ t('account.sessions.thisDevice') }}
          </Badge>
        </div>
        <dl class="mt-4 grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 text-xs">
          <dt class="text-muted-foreground">
            {{ t('account.sessions.device') }}
          </dt>
          <dd class="break-all">
            {{ deviceLabel(currentSession.userAgent) }}
          </dd>
          <dt class="text-muted-foreground">
            {{ t('account.sessions.ip') }}
          </dt>
          <dd>{{ currentSession.ip ?? t('account.sessions.unknownIp') }}</dd>
          <dt class="text-muted-foreground">
            {{ t('account.sessions.created') }}
          </dt>
          <dd>{{ formatDate(currentSession.createdAt) }}</dd>
          <dt class="text-muted-foreground">
            {{ t('account.sessions.lastSeen') }}
          </dt>
          <dd>{{ formatDate(currentSession.lastSeenAt) }}</dd>
          <dt class="text-muted-foreground">
            {{ t('account.sessions.expires') }}
          </dt>
          <dd>{{ formatDate(currentSession.expiresAt) }}</dd>
        </dl>
      </div>

      <div class="space-y-3">
        <div class="space-y-1">
          <h3 class="text-sm font-medium">
            {{ t('account.sessions.otherTitle') }}
          </h3>
          <p class="text-xs text-muted-foreground">
            {{ t('account.sessions.otherDescription') }}
          </p>
        </div>

        <p
          v-if="otherSessions.length === 0"
          class="rounded-lg border border-dashed px-4 py-6 text-center text-sm text-muted-foreground"
        >
          {{ t('account.sessions.empty') }}
        </p>

        <ul
          v-else
          class="divide-y rounded-lg border"
        >
          <li
            v-for="session in otherSessions"
            :key="session.id"
            class="flex flex-col gap-3 p-4 sm:flex-row sm:items-start sm:justify-between"
          >
            <div class="space-y-2">
              <p class="break-all text-sm font-medium">
                {{ deviceLabel(session.userAgent) }}
              </p>
              <dl class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-xs">
                <dt class="text-muted-foreground">
                  {{ t('account.sessions.ip') }}
                </dt>
                <dd>{{ session.ip ?? t('account.sessions.unknownIp') }}</dd>
                <dt class="text-muted-foreground">
                  {{ t('account.sessions.created') }}
                </dt>
                <dd>{{ formatDate(session.createdAt) }}</dd>
                <dt class="text-muted-foreground">
                  {{ t('account.sessions.lastSeen') }}
                </dt>
                <dd>{{ formatDate(session.lastSeenAt) }}</dd>
                <dt class="text-muted-foreground">
                  {{ t('account.sessions.expires') }}
                </dt>
                <dd>{{ formatDate(session.expiresAt) }}</dd>
              </dl>
            </div>
            <Button
              variant="outline"
              size="sm"
              :disabled="revokePending && variables === session.id"
              @click="revokeSession(session.id)"
            >
              {{
                revokePending && variables === session.id
                  ? t('account.sessions.revoking')
                  : t('account.sessions.revoke')
              }}
            </Button>
          </li>
        </ul>
      </div>
    </template>
  </section>
</template>
