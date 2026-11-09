<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { isAppLocale, setLocale, type AppLocale } from '@/i18n'
import { useSession } from '@/features/auth/queries'
import { useSignOut } from '@/features/auth/use-sign-out'

const { t, locale } = useI18n()
const { data: session } = useSession()
const { signOut, isPending: signingOut } = useSignOut()

const isSignedIn = computed(() => session.value !== null)
const isAdmin = computed(() => session.value?.isAdmin === true)

const navLinkClass =
  'rounded-md px-3 py-2 text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground'

function changeLocale(value: unknown) {
  if (typeof value === 'string' && isAppLocale(value)) {
    setLocale(value)
  }
}

function currentLocale(): AppLocale {
  return isAppLocale(locale.value) ? locale.value : 'en'
}
</script>

<template>
  <div class="flex min-h-screen flex-col bg-background text-foreground">
    <header class="border-b bg-card/80 backdrop-blur">
      <div
        class="mx-auto flex w-full max-w-5xl flex-wrap items-center justify-between gap-4 px-6 py-4"
      >
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
        <nav class="flex flex-wrap items-center gap-1 text-sm">
          <RouterLink
            to="/"
            :class="navLinkClass"
            active-class="bg-accent text-accent-foreground"
          >
            {{ t('nav.home') }}
          </RouterLink>
          <RouterLink
            v-if="isSignedIn"
            to="/account"
            :class="navLinkClass"
            active-class="bg-accent text-accent-foreground"
          >
            {{ t('nav.account') }}
          </RouterLink>
          <RouterLink
            v-if="isAdmin"
            to="/admin"
            :class="navLinkClass"
            active-class="bg-accent text-accent-foreground"
          >
            {{ t('nav.admin') }}
          </RouterLink>
          <RouterLink
            v-if="!isSignedIn"
            to="/login"
            :class="navLinkClass"
            active-class="bg-accent text-accent-foreground"
          >
            {{ t('nav.login') }}
          </RouterLink>
          <Button
            v-if="isSignedIn"
            variant="ghost"
            size="sm"
            :disabled="signingOut"
            @click="signOut"
          >
            {{ signingOut ? t('nav.signingOut') : t('nav.logout') }}
          </Button>
          <label
            class="sr-only"
            for="locale-switch"
          >
            {{ t('nav.language') }}
          </label>
          <Select
            :model-value="currentLocale()"
            @update:model-value="changeLocale"
          >
            <SelectTrigger
              id="locale-switch"
              class="h-8 w-20 text-xs"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="en">
                EN
              </SelectItem>
              <SelectItem value="id">
                ID
              </SelectItem>
            </SelectContent>
          </Select>
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
