import type { QueryClient } from '@tanstack/vue-query'

import { authKeys } from './query-keys'

function isSessionQuery(key: readonly unknown[]): boolean {
  return key[0] === authKeys.session[0] && key[1] === authKeys.session[1]
}

// Keep the session query subscribed so the header reacts, and drop every other
// cached response on sign-out. Removing the session query detaches its
// observers and leaves them showing the previous user.
export function clearAuthCache(queryClient: QueryClient) {
  queryClient.removeQueries({
    predicate: (query) => !isSessionQuery(query.queryKey),
  })
  queryClient.setQueryData(authKeys.session, null)
}
