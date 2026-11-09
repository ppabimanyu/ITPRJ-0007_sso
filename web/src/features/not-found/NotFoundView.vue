<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

const { t } = useI18n()
</script>

<template>
  <section class="space-y-8">
    <p class="text-xs font-medium uppercase tracking-[0.2em] text-muted-foreground">
      404
    </p>
    <Card class="max-w-2xl">
      <CardHeader>
        <CardTitle>{{ t('notFound.title') }}</CardTitle>
        <CardDescription>{{ t('notFound.description') }}</CardDescription>
      </CardHeader>
      <CardContent>
        <Button as-child>
          <RouterLink to="/">{{ t('notFound.backHome') }}</RouterLink>
        </Button>
      </CardContent>
    </Card>
  </section>
</template>
