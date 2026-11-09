<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import { Badge } from '@/components/ui/badge'

defineProps<{
  clientName: string
  scopes: string[]
}>()

const { t } = useI18n()
</script>

<template>
  <div class="rounded-lg border bg-muted/40 p-4">
    <dl class="space-y-3">
      <div class="flex flex-wrap items-baseline justify-between gap-2">
        <dt class="text-xs font-medium uppercase tracking-wide text-muted-foreground">
          {{ t('auth.authorize.clientLabel') }}
        </dt>
        <dd class="text-sm font-semibold">
          {{ clientName }}
        </dd>
      </div>
      <div class="space-y-2">
        <dt class="text-xs font-medium uppercase tracking-wide text-muted-foreground">
          {{ t('auth.authorize.scopesLabel') }}
        </dt>
        <dd class="flex flex-wrap gap-2">
          <Badge
            v-for="scope in scopes"
            :key="scope"
            variant="secondary"
          >
            {{ scope }}
          </Badge>
        </dd>
      </div>
    </dl>
  </div>
</template>
