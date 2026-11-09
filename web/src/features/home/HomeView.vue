<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { fetchApiHealth } from '@/lib/api'

const { t } = useI18n()

const { data, error, isError, isFetching, isPending, refetch } = useQuery({
  queryKey: ['api-health'],
  queryFn: fetchApiHealth,
})

const stack = [
  'Vue 3',
  'Vite',
  'Vue Router',
  'TanStack Query',
  'vue-i18n',
  'Tailwind CSS',
  'shadcn-vue',
]

const status = computed(() => {
  if (isPending.value) {
    return { label: t('home.api.checking'), tone: 'bg-amber-500' }
  }
  if (isError.value) {
    return { label: t('home.api.unavailable'), tone: 'bg-destructive' }
  }
  return { label: t('home.api.ready'), tone: 'bg-emerald-500' }
})

const errorMessage = computed(() => {
  if (!isError.value) {
    return ''
  }
  return error.value instanceof Error ? error.value.message : String(error.value)
})
</script>

<template>
  <section class="space-y-8">
    <div class="space-y-3">
      <p class="text-xs font-medium uppercase tracking-[0.2em] text-muted-foreground">
        {{ t('home.eyebrow') }}
      </p>
      <h1 class="text-3xl font-semibold tracking-tight sm:text-4xl">
        {{ t('app.name') }}
      </h1>
      <p class="max-w-2xl text-muted-foreground">
        {{ t('home.description') }}
      </p>
    </div>

    <div class="grid gap-4 md:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle class="flex items-center justify-between gap-3">
            <span>{{ t('home.api.title') }}</span>
            <span
              class="inline-flex items-center gap-2 rounded-full border px-2.5 py-1 text-xs font-medium text-muted-foreground"
            >
              <span
                class="h-2 w-2 rounded-full"
                :class="status.tone"
              />
              {{ status.label }}
            </span>
          </CardTitle>
          <CardDescription>{{ t('home.api.description') }}</CardDescription>
        </CardHeader>
        <CardContent class="space-y-4">
          <p
            v-if="isPending"
            class="text-muted-foreground"
          >
            {{ t('home.api.checking') }}…
          </p>

          <div
            v-else-if="isError"
            class="space-y-2"
          >
            <p class="font-medium text-destructive">
              {{ t('home.api.unavailable') }}
            </p>
            <p class="font-mono text-xs text-muted-foreground">
              {{ errorMessage }}
            </p>
          </div>

          <div
            v-else-if="data"
            class="space-y-3"
          >
            <p
              v-if="!data.version"
              class="text-muted-foreground"
            >
              {{ t('home.api.fallback') }}
            </p>
            <dl class="grid grid-cols-[auto_1fr] items-baseline gap-x-6 gap-y-2 text-sm">
              <dt class="text-muted-foreground">
                {{ t('home.api.source') }}
              </dt>
              <dd class="font-mono text-xs">
                {{ data.endpoint }}
              </dd>
              <template v-if="data.version">
                <dt class="text-muted-foreground">
                  {{ t('home.api.version') }}
                </dt>
                <dd class="font-mono text-xs">
                  {{ data.version.version }}
                </dd>
                <dt class="text-muted-foreground">
                  {{ t('home.api.commit') }}
                </dt>
                <dd class="font-mono text-xs">
                  {{ data.version.commit || 'unknown' }}
                </dd>
                <dt class="text-muted-foreground">
                  {{ t('home.api.goVersion') }}
                </dt>
                <dd class="font-mono text-xs">
                  {{ data.version.goVersion || 'unknown' }}
                </dd>
              </template>
            </dl>
          </div>

          <Button
            variant="outline"
            size="sm"
            :disabled="isFetching"
            @click="refetch()"
          >
            {{ t('home.api.retry') }}
          </Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{{ t('home.stack.title') }}</CardTitle>
          <CardDescription>{{ t('home.stack.description') }}</CardDescription>
        </CardHeader>
        <CardContent>
          <ul class="grid grid-cols-2 gap-2 text-sm text-muted-foreground">
            <li
              v-for="item in stack"
              :key="item"
              class="rounded-md bg-muted px-3 py-2"
            >
              {{ item }}
            </li>
          </ul>
        </CardContent>
      </Card>
    </div>
  </section>
</template>
