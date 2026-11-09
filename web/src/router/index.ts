import { createRouter, createWebHistory } from 'vue-router'

import { i18n } from '@/i18n'
import { fetchSession } from '@/lib/auth-api'
import { queryClient } from '@/lib/query'
import { authKeys } from '@/lib/query-keys'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'home',
      component: () => import('@/features/home/HomeView.vue'),
      meta: { titleKey: 'nav.home' },
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('@/features/auth/LoginView.vue'),
      meta: { titleKey: 'auth.login.title' },
    },
    {
      path: '/consent',
      name: 'consent',
      component: () => import('@/features/auth/ConsentView.vue'),
      meta: { titleKey: 'auth.consent.title' },
    },
    {
      path: '/forgot-password',
      name: 'forgot-password',
      component: () => import('@/features/auth/ForgotPasswordView.vue'),
      meta: { titleKey: 'auth.forgot.title' },
    },
    {
      path: '/reset-password',
      name: 'reset-password',
      component: () => import('@/features/auth/ResetPasswordView.vue'),
      meta: { titleKey: 'auth.reset.title' },
    },
    {
      path: '/accept-invite',
      name: 'accept-invite',
      component: () => import('@/features/auth/AcceptInviteView.vue'),
      meta: { titleKey: 'auth.invite.title' },
    },
    {
      path: '/verify-email',
      name: 'verify-email',
      component: () => import('@/features/auth/VerifyEmailView.vue'),
      meta: { titleKey: 'auth.verify.title' },
    },
    {
      path: '/account',
      name: 'account',
      component: () => import('@/features/account/AccountView.vue'),
      meta: { titleKey: 'account.title', requiresAuth: true },
    },
    {
      path: '/mfa/enroll',
      name: 'mfa-enroll',
      component: () => import('@/features/mfa/MfaEnrollView.vue'),
      meta: { titleKey: 'auth.enroll.title', requiresAuth: true },
    },
    {
      path: '/admin',
      component: () => import('@/features/admin/AdminView.vue'),
      meta: { titleKey: 'admin.title', requiresAdmin: true },
      children: [
        {
          path: '',
          name: 'admin-dashboard',
          component: () => import('@/features/admin/dashboard/AdminDashboardView.vue'),
          meta: { titleKey: 'admin.title', requiresAdmin: true },
        },
        {
          path: 'users',
          name: 'admin-users',
          component: () => import('@/features/admin/users/AdminUsersView.vue'),
          meta: { titleKey: 'admin.users.title', requiresAdmin: true },
        },
        {
          path: 'users/:id',
          name: 'admin-user',
          component: () => import('@/features/admin/users/AdminUserDetailView.vue'),
          meta: { titleKey: 'admin.users.detailTitle', requiresAdmin: true },
        },
        {
          path: 'clients',
          name: 'admin-clients',
          component: () => import('@/features/admin/clients/AdminClientsView.vue'),
          meta: { titleKey: 'admin.clients.title', requiresAdmin: true },
        },
        {
          path: 'clients/:id',
          name: 'admin-client',
          component: () => import('@/features/admin/clients/AdminClientDetailView.vue'),
          meta: { titleKey: 'admin.clients.detailTitle', requiresAdmin: true },
        },
        {
          path: 'roles',
          name: 'admin-roles',
          component: () => import('@/features/admin/roles/AdminRolesView.vue'),
          meta: { titleKey: 'admin.roles.title', requiresAdmin: true },
        },
        {
          path: 'groups',
          name: 'admin-groups',
          component: () => import('@/features/admin/groups/AdminGroupsView.vue'),
          meta: { titleKey: 'admin.groups.title', requiresAdmin: true },
        },
        {
          path: 'groups/:id',
          name: 'admin-group',
          component: () => import('@/features/admin/groups/AdminGroupDetailView.vue'),
          meta: { titleKey: 'admin.groups.detailTitle', requiresAdmin: true },
        },
        {
          path: 'sessions',
          name: 'admin-sessions',
          component: () => import('@/features/admin/sessions/AdminSessionsView.vue'),
          meta: { titleKey: 'admin.sessions.title', requiresAdmin: true },
        },
        {
          path: 'audit',
          name: 'admin-audit',
          component: () => import('@/features/admin/audit/AdminAuditView.vue'),
          meta: { titleKey: 'admin.audit.title', requiresAdmin: true },
        },
        {
          path: 'settings',
          name: 'admin-settings',
          component: () => import('@/features/admin/settings/AdminSettingsView.vue'),
          meta: { titleKey: 'admin.settings.title', requiresAdmin: true },
        },
      ],
    },
    {
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      component: () => import('@/features/not-found/NotFoundView.vue'),
      meta: { titleKey: 'notFound.title' },
    },
  ],
})

router.beforeEach(async (to) => {
  if (!to.meta.requiresAuth && !to.meta.requiresAdmin) {
    return true
  }
  let session
  try {
    session = await queryClient.fetchQuery({
      queryKey: authKeys.session,
      queryFn: fetchSession,
      staleTime: 30_000,
    })
  } catch {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (session === null) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (session.isAdmin && !session.mfaEnrolled && to.name !== 'mfa-enroll') {
    return {
      name: 'mfa-enroll',
      query: to.fullPath === '/' ? {} : { redirect: to.fullPath },
    }
  }
  if (to.meta.requiresAdmin && !session.isAdmin) {
    return { name: 'account' }
  }
  return true
})

router.afterEach((to) => {
  const titleKey = to.meta.titleKey ?? 'app.name'
  document.title = `${i18n.global.t(titleKey)} · ${i18n.global.t('app.name')}`
})

export default router
