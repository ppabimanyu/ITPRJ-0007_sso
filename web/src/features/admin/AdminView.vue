<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

const { t } = useI18n()
</script>

<template>
  <section class="space-y-8">
    <div class="space-y-3">
      <p class="text-xs font-medium uppercase tracking-[0.2em] text-muted-foreground">
        {{ t('nav.admin') }}
      </p>
      <h1 class="text-3xl font-semibold tracking-tight sm:text-4xl">
        {{ t('admin.title') }}
      </h1>
    </div>

    <Card class="max-w-2xl">
      <CardHeader>
        <CardTitle>{{ t('admin.title') }}</CardTitle>
        <CardDescription>{{ t('admin.description') }}</CardDescription>
      </CardHeader>
      <CardContent>
        <Button
          as-child
          variant="outline"
        >
          <RouterLink to="/">{{ t('admin.backHome') }}</RouterLink>
        </Button>
      </CardContent>
    </Card>
  </section>
</template>
