import { useMutation, useQuery } from '@tanstack/vue-query'
import { computed, toValue, type MaybeRefOrGetter } from 'vue'

import {
  completeAuthorize,
  fetchAuthorizeRequest,
  grantConsent,
  type AuthorizeRequestId,
  type ConsentInput,
} from '@/lib/oidc-api'
import { oidcKeys } from '@/lib/query-keys'

export function useAuthorizeRequest(requestId: MaybeRefOrGetter<AuthorizeRequestId | null>) {
  return useQuery({
    queryKey: computed(() => oidcKeys.authorizeRequest(toValue(requestId) ?? 'missing')),
    queryFn: () => {
      const id = toValue(requestId)
      if (id === null) {
        throw new Error('authorize request id is missing')
      }
      return fetchAuthorizeRequest(id)
    },
    enabled: () => toValue(requestId) !== null,
    retry: false,
  })
}

export function useCompleteAuthorize() {
  return useMutation({
    mutationFn: (requestId: AuthorizeRequestId) => completeAuthorize(requestId),
  })
}

export function useGrantConsent() {
  return useMutation({
    mutationFn: (input: ConsentInput) => grantConsent(input),
  })
}
