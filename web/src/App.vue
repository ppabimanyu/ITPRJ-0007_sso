<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
</script>

<template>
  <div class="flex min-h-screen flex-col bg-background text-foreground">
    <header class="border-b bg-card/80 backdrop-blur">
      <div class="mx-auto flex w-full max-w-5xl items-center justify-between gap-4 px-6 py-4">
        <RouterLink
          to="/"
          class="flex items-center gap-3"
        >
          <span
            class="grid h-9 w-9 place-items-center rounded-lg bg-primary text-sm font-semibold text-primary-foreground"
          >
            S
          </span>
          <span class="text-sm font-semibold tracking-tight">{{ t('app.name') }}</span>
        </RouterLink>
        <nav class="flex items-center gap-1 text-sm">
          <RouterLink
            to="/"
            class="rounded-md px-3 py-2 text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
            active-class="bg-accent text-accent-foreground"
          >
            {{ t('nav.home') }}
          </RouterLink>
          <RouterLink
            to="/admin"
            class="rounded-md px-3 py-2 text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
            active-class="bg-accent text-accent-foreground"
          >
            {{ t('nav.admin') }}
          </RouterLink>
        </nav>
      </div>
    </header>

    <main class="mx-auto w-full max-w-5xl flex-1 px-6 py-12">
      <RouterView />
    </main>

    <footer class="border-t">
      <p class="mx-auto w-full max-w-5xl px-6 py-6 text-xs text-muted-foreground">
        {{ t('app.footer') }}
      </p>
    </footer>
  </div>
</template>
