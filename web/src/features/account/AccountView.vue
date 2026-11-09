<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'

import { useSession } from '../auth/queries'
import { useSignOut } from '../auth/use-sign-out'
import PasswordForm from './PasswordForm.vue'
import ProfileForm from './ProfileForm.vue'
import SessionPanel from './SessionPanel.vue'

const { t } = useI18n()
const { data: session, isPending } = useSession()
const { signOut, isPending: signingOut } = useSignOut()

const displayName = computed(() => {
  const user = session.value?.user
  if (!user) {
    return ''
  }
  if (user.name !== null && user.name !== '') {
    return user.name
  }
  const parts = [user.givenName, user.familyName].filter((part): part is string => part !== null)
  return parts.length > 0 ? parts.join(' ') : user.email
})
</script>

<template>
  <section
    v-if="session"
    class="space-y-8"
  >
    <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
      <div class="space-y-2">
        <p class="text-xs font-medium uppercase tracking-[0.2em] text-muted-foreground">
          {{ t('account.eyebrow') }}
        </p>
        <h1 class="text-3xl font-semibold tracking-tight sm:text-4xl">
          {{ t('account.title') }}
        </h1>
        <p class="text-muted-foreground">
          {{ displayName }} · {{ session.user.email }}
        </p>
        <div
          v-if="session.user.roles.length > 0"
          class="flex flex-wrap items-center gap-2"
        >
          <span class="text-xs text-muted-foreground">{{ t('account.roles') }}</span>
          <Badge
            v-for="role in session.user.roles"
            :key="role.id"
            variant="outline"
          >
            {{ role.name }}
          </Badge>
        </div>
      </div>

      <Button
        variant="outline"
        :disabled="signingOut"
        @click="signOut"
      >
        {{ signingOut ? t('nav.signingOut') : t('nav.logout') }}
      </Button>
    </div>

    <Card>
      <CardContent class="pt-6">
        <ProfileForm :user="session.user" />
      </CardContent>
    </Card>

    <Card>
      <CardContent class="pt-6">
        <PasswordForm />
      </CardContent>
    </Card>

    <Card>
      <CardContent class="pt-6">
        <SessionPanel />
      </CardContent>
    </Card>
  </section>

  <section
    v-else-if="isPending"
    class="text-sm text-muted-foreground"
  >
    {{ t('common.loading') }}
  </section>

  <Alert
    v-else
    variant="destructive"
  >
    <AlertTitle>{{ t('errors.unauthorized') }}</AlertTitle>
    <AlertDescription>
      <RouterLink
        to="/login"
        class="font-medium underline underline-offset-4"
      >
        {{ t('nav.login') }}
      </RouterLink>
    </AlertDescription>
  </Alert>
</template>
