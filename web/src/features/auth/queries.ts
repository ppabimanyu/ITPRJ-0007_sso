import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'

import {
  acceptInvite,
  changePassword,
  fetchSession,
  forgotPassword,
  listSessions,
  login,
  logout,
  resetPassword,
  revokeSession,
  updateProfile,
  verifyEmail,
} from '@/lib/auth-api'
import { clearAuthCache } from '@/lib/auth-cache'
import { authKeys } from '@/lib/query-keys'

async function reloadSession(queryClient: ReturnType<typeof useQueryClient>) {
  await queryClient.fetchQuery({
    queryKey: authKeys.session,
    queryFn: fetchSession,
    staleTime: 0,
  })
}

export function useSession() {
  return useQuery({
    queryKey: authKeys.session,
    queryFn: fetchSession,
    retry: false,
    staleTime: 30_000,
  })
}

export function useLogin() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: login,
    onSuccess: async (result) => {
      if (result.status === 'authenticated') {
        await reloadSession(queryClient)
      }
    },
  })
}

export function useLogout() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: logout,
    onSuccess: () => {
      clearAuthCache(queryClient)
    },
  })
}

export function useForgotPassword() {
  return useMutation({ mutationFn: forgotPassword })
}

export function useResetPassword() {
  return useMutation({
    mutationFn: (input: { token: string; newPassword: string }) =>
      resetPassword(input.token, input.newPassword),
  })
}

export function useVerifyEmail() {
  return useMutation({ mutationFn: verifyEmail })
}

export function useAcceptInvite() {
  return useMutation({
    mutationFn: (input: { token: string; password: string }) =>
      acceptInvite(input.token, input.password),
  })
}

export function useUpdateProfile() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: updateProfile,
    onSuccess: async () => {
      await reloadSession(queryClient)
    },
  })
}

export function useChangePassword() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: changePassword,
    onSuccess: async () => {
      await reloadSession(queryClient)
      await queryClient.invalidateQueries({ queryKey: authKeys.sessions })
    },
  })
}

export function useSessions() {
  return useQuery({
    queryKey: authKeys.sessions,
    queryFn: listSessions,
    retry: 1,
  })
}

export function useRevokeSession() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: revokeSession,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: authKeys.sessions })
    },
  })
}
