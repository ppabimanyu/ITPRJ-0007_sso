import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

export function useAdminFormat() {
  const { locale } = useI18n()
  const dateFormat = computed(
    () => new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }),
  )
  const compactFormat = computed(
    () =>
      new Intl.DateTimeFormat(locale.value, {
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      }),
  )
  const compactFormatWithYear = computed(
    () =>
      new Intl.DateTimeFormat(locale.value, {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      }),
  )

  function formatDate(value: string | null): string {
    if (value === null || value === '') {
      return ''
    }
    const date = new Date(value)
    return Number.isNaN(date.getTime()) ? value : dateFormat.value.format(date)
  }

  function formatCompactDate(value: string | null): string {
    if (value === null || value === '') {
      return ''
    }
    const date = new Date(value)
    if (Number.isNaN(date.getTime())) {
      return value
    }
    const format =
      date.getFullYear() === new Date().getFullYear() ? compactFormat.value : compactFormatWithYear.value
    return format.format(date)
  }

  return { formatDate, formatCompactDate }
}
