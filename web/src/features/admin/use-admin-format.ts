import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

export function useAdminFormat() {
  const { locale } = useI18n()
  const dateFormat = computed(
    () => new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }),
  )

  function formatDate(value: string | null): string {
    if (value === null || value === '') {
      return ''
    }
    const date = new Date(value)
    return Number.isNaN(date.getTime()) ? value : dateFormat.value.format(date)
  }

  return { formatDate }
}
