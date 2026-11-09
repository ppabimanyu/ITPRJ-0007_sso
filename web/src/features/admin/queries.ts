import { useMutation, useQueryClient } from '@tanstack/vue-query'

import {
  addGroupMembers,
  assignUserRoles,
  createClient,
  createGroup,
  createRole,
  createUser,
  deleteClient,
  deleteGroup,
  removeGroupMember,
  resendUserInvite,
  resetUserPassword,
  revokeAdminSession,
  rotateClientSecret,
  updateClient,
  updateGroup,
  updateRealmSettings,
  updateUser,
  type CreateClientInput,
  type CreateGroupInput,
  type CreateRoleInput,
  type CreateUserInput,
  type GroupWriteInput,
  type UpdateClientInput,
  type UpdateRealmSettingsInput,
  type UpdateUserInput,
} from '@/lib/admin/api'
import { authKeys, adminKeys } from '@/lib/query-keys'

export function useCreateUser() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: CreateUserInput) => createUser(input),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: adminKeys.users })
    },
  })
}

export function useUpdateUser() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: { id: string; values: UpdateUserInput }) =>
      updateUser(input.id, input.values),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: adminKeys.users })
      await queryClient.invalidateQueries({ queryKey: authKeys.session })
    },
  })
}

export function useResetUserPassword() {
  return useMutation({
    mutationFn: (id: string) => resetUserPassword(id),
  })
}

export function useResendUserInvite() {
  return useMutation({
    mutationFn: (id: string) => resendUserInvite(id),
  })
}

export function useAssignUserRoles() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: { id: string; roleIds: string[] }) =>
      assignUserRoles(input.id, input.roleIds),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: adminKeys.users })
      await queryClient.invalidateQueries({ queryKey: authKeys.session })
    },
  })
}

export function useCreateClient() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: CreateClientInput) => createClient(input),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: adminKeys.clients })
    },
  })
}

export function useUpdateClient() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: { id: string; values: UpdateClientInput }) =>
      updateClient(input.id, input.values),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: adminKeys.clients })
    },
  })
}

export function useDeleteClient() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => deleteClient(id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: adminKeys.clients })
    },
  })
}

export function useRotateClientSecret() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => rotateClientSecret(id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: adminKeys.clients })
    },
  })
}

export function useCreateRole() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: CreateRoleInput) => createRole(input),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: adminKeys.roles })
    },
  })
}

export function useCreateGroup() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: CreateGroupInput) => createGroup(input),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: adminKeys.groups })
    },
  })
}

export function useUpdateGroup() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: { id: string; values: GroupWriteInput }) =>
      updateGroup(input.id, input.values),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: adminKeys.groups })
    },
  })
}

export function useDeleteGroup() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => deleteGroup(id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: adminKeys.groups })
    },
  })
}

export function useAddGroupMembers() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: { id: string; userIds: string[] }) =>
      addGroupMembers(input.id, input.userIds),
    onSuccess: async (_data, input) => {
      await queryClient.invalidateQueries({ queryKey: adminKeys.groupMembers(input.id) })
      await queryClient.invalidateQueries({ queryKey: adminKeys.groups })
    },
  })
}

export function useRemoveGroupMember() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: { id: string; userId: string }) =>
      removeGroupMember(input.id, input.userId),
    onSuccess: async (_data, input) => {
      await queryClient.invalidateQueries({ queryKey: adminKeys.groupMembers(input.id) })
      await queryClient.invalidateQueries({ queryKey: adminKeys.groups })
    },
  })
}

export function useRevokeAdminSession() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => revokeAdminSession(id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: adminKeys.sessions })
    },
  })
}

export function useUpdateRealmSettings() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: UpdateRealmSettingsInput) => updateRealmSettings(input),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: adminKeys.settings })
    },
  })
}
