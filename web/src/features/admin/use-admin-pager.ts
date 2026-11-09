import { computed, ref } from 'vue'

export function useAdminPager() {
  const after = ref<string | null>(null)
  const history = ref<string[]>([])
  const hasPrevious = computed(() => history.value.length > 0)

  function next(nextAfter: string | null) {
    if (nextAfter === null) {
      return
    }
    history.value.push(after.value ?? '')
    after.value = nextAfter
  }

  function previous() {
    const value = history.value.pop()
    after.value = value === undefined || value === '' ? null : value
  }

  function reset() {
    after.value = null
    history.value = []
  }

  return { after, hasPrevious, next, previous, reset }
}
