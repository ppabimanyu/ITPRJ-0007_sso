<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { useSession } from '../auth/queries'
import { useSignOut } from '../auth/use-sign-out'
import MfaPanel from './MfaPanel.vue'
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
    <div class="grid gap-5 rounded-lg border bg-card p-6 sm:grid-cols-[auto_minmax(0,1fr)_auto] sm:items-center">
      <div class="seal h-14 w-14 text-2xl">
        S
      </div>
      <div class="space-y-1">
        <p class="kicker">
          {{ t('account.eyebrow') }}
        </p>
        <h1 class="font-display text-3xl sm:text-4xl">
          {{ t('account.title') }}
        </h1>
        <p class="text-muted-foreground">
          {{ displayName }}
        </p>
        <p class="text-sm text-muted-foreground">
          {{ session.user.email }}
        </p>
        <div
          v-if="session.user.roles.length > 0"
          class="flex flex-wrap items-center gap-2 pt-1"
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

    <Tabs default-value="profile">
      <TabsList
        class="h-auto w-full justify-start gap-0 overflow-x-auto rounded-none border-b border-border/70 bg-transparent p-0"
      >
        <TabsTrigger
          value="profile"
          class="h-auto shrink-0 rounded-none border-b-2 border-transparent px-4 py-3 text-sm text-muted-foreground shadow-none data-[state=active]:border-b-brass data-[state=active]:bg-transparent data-[state=active]:text-foreground data-[state=active]:shadow-none"
        >
          {{ t('account.profile.title') }}
        </TabsTrigger>
        <TabsTrigger
          value="password"
          class="h-auto shrink-0 rounded-none border-b-2 border-transparent px-4 py-3 text-sm text-muted-foreground shadow-none data-[state=active]:border-b-brass data-[state=active]:bg-transparent data-[state=active]:text-foreground data-[state=active]:shadow-none"
        >
          {{ t('account.password.title') }}
        </TabsTrigger>
        <TabsTrigger
          value="mfa"
          class="h-auto shrink-0 rounded-none border-b-2 border-transparent px-4 py-3 text-sm text-muted-foreground shadow-none data-[state=active]:border-b-brass data-[state=active]:bg-transparent data-[state=active]:text-foreground data-[state=active]:shadow-none"
        >
          {{ t('account.mfa.tabLabel') }}
        </TabsTrigger>
        <TabsTrigger
          value="sessions"
          class="h-auto shrink-0 rounded-none border-b-2 border-transparent px-4 py-3 text-sm text-muted-foreground shadow-none data-[state=active]:border-b-brass data-[state=active]:bg-transparent data-[state=active]:text-foreground data-[state=active]:shadow-none"
        >
          {{ t('account.sessions.title') }}
        </TabsTrigger>
      </TabsList>

      <TabsContent value="profile">
        <Card>
          <CardContent class="pt-6">
            <ProfileForm :user="session.user" />
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="password">
        <Card>
          <CardContent class="pt-6">
            <PasswordForm />
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="mfa">
        <Card>
          <CardContent class="pt-6">
            <MfaPanel
              :enrolled="session.user.mfaEnrolled"
              :is-admin="session.isAdmin"
            />
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="sessions">
        <Card>
          <CardContent class="pt-6">
            <SessionPanel />
          </CardContent>
        </Card>
      </TabsContent>
    </Tabs>
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
