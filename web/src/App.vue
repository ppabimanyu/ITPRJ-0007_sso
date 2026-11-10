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
    <header class="sticky top-0 z-40 border-b border-border/80 bg-background/90 backdrop-blur">
      <div
        class="mx-auto flex w-full max-w-6xl flex-wrap items-center justify-between gap-3 px-5 py-3 sm:px-8"
      >
        <RouterLink
          to="/"
          class="flex items-center gap-3"
        >
          <span class="seal">S</span>
          <span class="flex flex-col leading-tight">
            <span class="font-display text-lg">{{ t('app.name') }}</span>
            <span class="text-[11px] text-muted-foreground">{{ t('app.tagline') }}</span>
          </span>
        </RouterLink>
        <nav class="flex flex-wrap items-center gap-1 text-sm">
          <RouterLink
            to="/"
            class="nav-link"
            exact-active-class="nav-link-active"
          >
            {{ t('nav.home') }}
          </RouterLink>
          <RouterLink
            v-if="isSignedIn"
            to="/account"
            class="nav-link"
            active-class="nav-link-active"
          >
            {{ t('nav.account') }}
          </RouterLink>
          <RouterLink
            v-if="isAdmin"
            to="/admin"
            class="nav-link"
            active-class="nav-link-active"
          >
            {{ t('nav.admin') }}
          </RouterLink>
          <RouterLink
            v-if="!isSignedIn"
            to="/login"
            class="nav-link"
            active-class="nav-link-active"
          >
            {{ t('nav.login') }}
          </RouterLink>
          <span
            v-if="isSignedIn"
            class="mx-1 hidden h-5 w-px bg-border sm:block"
            aria-hidden="true"
          />
          <Button
            v-if="isSignedIn"
            variant="ghost"
            size="sm"
            class="text-muted-foreground"
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
              class="h-8 w-16 text-xs"
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

    <main class="mx-auto w-full max-w-6xl flex-1 px-5 py-10 sm:px-8 sm:py-14">
      <div
        :key="$route.fullPath"
        class="page-reveal"
      >
        <RouterView />
      </div>
    </main>

    <footer class="border-t border-border/70">
      <p class="mx-auto w-full max-w-6xl px-5 py-6 text-xs text-muted-foreground sm:px-8">
        {{ t('app.footer') }}
      </p>
    </footer>
  </div>
</template>
