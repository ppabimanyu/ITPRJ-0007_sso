import { useRouter } from 'vue-router'

import { useLogout } from './queries'

export function useSignOut() {
  const router = useRouter()
  const { mutateAsync, isPending } = useLogout()

  async function signOut() {
    try {
      await mutateAsync()
    } catch {
      // A session-expired handler already sends the user to login.
    }
    await router.replace({ name: 'login' })
  }

  return { signOut, isPending }
}
