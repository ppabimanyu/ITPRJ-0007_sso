<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { useSession } from '@/features/auth/queries'
import { fetchApiHealth } from '@/lib/api'

const { t } = useI18n()
const { data: session } = useSession()

const isSignedIn = computed(() => session.value !== null)
const isAdmin = computed(() => session.value?.isAdmin === true)

const { data, error, isError, isFetching, isPending, refetch } = useQuery({
  queryKey: ['api-health'],
  queryFn: fetchApiHealth,
})

const services = computed(() => [
  { title: t('home.service.sso.title'), description: t('home.service.sso.description') },
  { title: t('home.service.mfa.title'), description: t('home.service.mfa.description') },
  {
    title: t('home.service.sessions.title'),
    description: t('home.service.sessions.description'),
  },
  { title: t('home.service.audit.title'), description: t('home.service.audit.description') },
])

const status = computed(() => {
  if (isPending.value) {
    return { label: t('home.api.checking'), tone: 'bg-brass' }
  }
  if (isError.value) {
    return { label: t('home.api.unavailable'), tone: 'bg-destructive' }
  }
  return { label: t('home.api.ready'), tone: 'bg-primary' }
})

const errorMessage = computed(() => {
  if (!isError.value) {
    return ''
  }
  return error.value instanceof Error ? error.value.message : String(error.value)
})
</script>

<template>
  <section class="space-y-14">
    <div class="grid gap-10 lg:grid-cols-[minmax(0,1fr)_360px] lg:items-start">
      <div class="space-y-6">
        <div class="seal h-14 w-14 text-2xl">
          S
        </div>
        <p class="kicker">
          {{ t('home.kicker') }}
        </p>
        <h1 class="font-display max-w-2xl text-4xl leading-[1.05] sm:text-6xl">
          {{ t('home.headline') }}
        </h1>
        <p class="max-w-xl text-lg text-muted-foreground">
          {{ t('home.body') }}
        </p>
        <div class="flex flex-wrap gap-3">
          <Button
            v-if="!isSignedIn"
            as-child
          >
            <RouterLink to="/login">{{ t('home.signIn') }}</RouterLink>
          </Button>
          <Button
            v-else
            as-child
          >
            <RouterLink to="/account">{{ t('home.openAccount') }}</RouterLink>
          </Button>
          <Button
            v-if="isAdmin"
            as-child
            variant="outline"
          >
            <RouterLink to="/admin">{{ t('home.adminConsole') }}</RouterLink>
          </Button>
        </div>
      </div>

      <Card class="gap-4 p-5">
        <div class="flex items-center justify-between gap-3">
          <p class="kicker">
            {{ t('home.api.title') }}
          </p>
          <span class="inline-flex items-center gap-2 text-xs text-muted-foreground">
            <span
              class="h-1.5 w-1.5 rounded-full"
              :class="status.tone"
            />
            {{ status.label }}
          </span>
        </div>
        <p class="text-sm text-muted-foreground">
          {{ t('home.api.description') }}
        </p>

        <p
          v-if="isPending"
          class="text-sm text-muted-foreground"
        >
          {{ t('home.api.checking') }}…
        </p>

        <div
          v-else-if="isError"
          class="space-y-1 border-t pt-3"
        >
          <p class="text-sm font-medium text-destructive">
            {{ t('home.api.unavailable') }}
          </p>
          <p class="font-mono text-xs break-all text-muted-foreground">
            {{ errorMessage }}
          </p>
        </div>

        <div
          v-else-if="data"
          class="border-t pt-3"
        >
          <p
            v-if="!data.version"
            class="text-sm text-muted-foreground"
          >
            {{ t('home.api.fallback') }}
          </p>
          <dl class="grid grid-cols-[auto_minmax(0,1fr)] items-baseline gap-x-4 gap-y-1 text-xs">
            <dt class="text-muted-foreground">
              {{ t('home.api.source') }}
            </dt>
            <dd class="font-mono break-all">
              {{ data.endpoint }}
            </dd>
            <template v-if="data.version">
              <dt class="text-muted-foreground">
                {{ t('home.api.version') }}
              </dt>
              <dd class="font-mono">
                {{ data.version.version }}
              </dd>
              <dt class="text-muted-foreground">
                {{ t('home.api.commit') }}
              </dt>
              <dd class="font-mono">
                {{ data.version.commit || 'unknown' }}
              </dd>
              <dt class="text-muted-foreground">
                {{ t('home.api.goVersion') }}
              </dt>
              <dd class="font-mono">
                {{ data.version.goVersion || 'unknown' }}
              </dd>
            </template>
          </dl>
        </div>

        <Button
          variant="outline"
          size="sm"
          class="self-start"
          :disabled="isFetching"
          @click="refetch()"
        >
          {{ t('home.api.retry') }}
        </Button>
      </Card>
    </div>

    <section class="space-y-5">
      <div class="border-b border-border/70 pb-4">
        <h2 class="font-display text-2xl">
          {{ t('home.servicesTitle') }}
        </h2>
        <p class="text-sm text-muted-foreground">
          {{ t('home.servicesDescription') }}
        </p>
      </div>
      <div class="grid gap-x-10 gap-y-6 sm:grid-cols-2">
        <article
          v-for="service in services"
          :key="service.title"
          class="border-t border-border/70 pt-4"
        >
          <h3 class="font-medium">
            {{ service.title }}
          </h3>
          <p class="mt-1 text-sm text-muted-foreground">
            {{ service.description }}
          </p>
        </article>
      </div>
    </section>
  </section>
</template>
