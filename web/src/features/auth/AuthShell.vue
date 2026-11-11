<script setup lang="ts">
import { Card, CardContent, CardDescription, CardHeader } from '@/components/ui/card'

defineProps<{
  title: string
  description: string
}>()
</script>

<template>
  <section class="mx-auto w-full max-w-md space-y-6">
    <Card>
      <CardHeader>
        <h1 class="text-lg font-semibold tracking-tight">{{ title }}</h1>
        <CardDescription>{{ description }}</CardDescription>
      </CardHeader>
      <CardContent class="space-y-5">
        <slot />
      </CardContent>
    </Card>
    <slot name="footer" />
  </section>
</template>
