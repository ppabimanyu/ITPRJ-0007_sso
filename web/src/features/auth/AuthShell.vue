<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

defineProps<{
  title: string
  description: string
}>()

const { t } = useI18n()
</script>

<template>
  <section class="mx-auto grid w-full max-w-5xl gap-8 lg:grid-cols-[minmax(0,1fr)_420px] lg:items-center">
    <div class="guilloche relative overflow-hidden rounded-lg border bg-secondary/40 p-8 sm:p-10">
      <div class="seal mb-6 h-14 w-14 text-2xl">
        S
      </div>
      <p class="kicker">
        {{ t('app.tagline') }}
      </p>
      <h2 class="font-display mt-3 max-w-md text-3xl leading-[1.1] sm:text-4xl">
        {{ t('auth.shell.headline') }}
      </h2>
      <p class="mt-4 max-w-md text-sm leading-relaxed text-muted-foreground">
        {{ t('auth.shell.body') }}
      </p>
      <ul class="mt-8 space-y-3 border-t pt-6 text-sm text-muted-foreground">
        <li class="flex gap-3">
          <span
            class="mt-1.5 h-1.5 w-1.5 flex-none rounded-full bg-brass"
            aria-hidden="true"
          />
          {{ t('auth.shell.factInternal') }}
        </li>
        <li class="flex gap-3">
          <span
            class="mt-1.5 h-1.5 w-1.5 flex-none rounded-full bg-brass"
            aria-hidden="true"
          />
          {{ t('auth.shell.factMfa') }}
        </li>
        <li class="flex gap-3">
          <span
            class="mt-1.5 h-1.5 w-1.5 flex-none rounded-full bg-brass"
            aria-hidden="true"
          />
          {{ t('auth.shell.factAudit') }}
        </li>
      </ul>
    </div>

    <div class="space-y-4">
      <Card class="border-t-2 border-t-primary">
        <CardHeader>
          <CardTitle class="font-display text-2xl">
            {{ title }}
          </CardTitle>
          <CardDescription>{{ description }}</CardDescription>
        </CardHeader>
        <CardContent class="space-y-5">
          <slot />
        </CardContent>
      </Card>
      <slot name="footer" />
    </div>
  </section>
</template>
