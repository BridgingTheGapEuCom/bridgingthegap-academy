import { useI18n } from 'vue-i18n'

export type ApplicationDateFormat = 'short' | 'date' | 'dateTime'
export type ApplicationNumberFormat = 'count' | 'decimal' | 'percent'

const dateFormats: Record<ApplicationDateFormat, Intl.DateTimeFormatOptions> = {
  short: { dateStyle: 'short' },
  date: { dateStyle: 'medium' },
  dateTime: { dateStyle: 'medium', timeStyle: 'short' },
}

const numberFormats: Record<ApplicationNumberFormat, Intl.NumberFormatOptions> = {
  count: { maximumFractionDigits: 0 },
  decimal: { maximumFractionDigits: 2 },
  percent: { style: 'percent', maximumFractionDigits: 1 },
}

export function formatApplicationDate(value: string | number | Date, locale: string, format: ApplicationDateFormat = 'date'): string {
  const date = value instanceof Date ? value : new Date(value)
  return Number.isNaN(date.getTime()) ? '' : new Intl.DateTimeFormat(locale, dateFormats[format]).format(date)
}

export function formatApplicationNumber(value: number, locale: string, format: ApplicationNumberFormat = 'decimal'): string {
  return new Intl.NumberFormat(locale, numberFormats[format]).format(value)
}

export function formatApplicationDuration(totalMinutes: number, locale: string): string {
  if (!Number.isSafeInteger(totalMinutes) || totalMinutes < 1) return ''
  const hours = Math.floor(totalMinutes / 60)
  const minutes = totalMinutes % 60
  const hourFormatter = new Intl.NumberFormat(locale, { style: 'unit', unit: 'hour', unitDisplay: 'short' })
  const minuteFormatter = new Intl.NumberFormat(locale, { style: 'unit', unit: 'minute', unitDisplay: 'short' })
  return [hours ? hourFormatter.format(hours) : '', minutes ? minuteFormatter.format(minutes) : ''].filter(Boolean).join(' ')
}

export function formatApplicationRelativeTime(value: number, unit: Intl.RelativeTimeFormatUnit, locale: string): string {
  return new Intl.RelativeTimeFormat(locale, { numeric: 'auto' }).format(value, unit)
}

export function useApplicationFormatters() {
  const { locale } = useI18n()
  return {
    date: (value: string | number | Date, format?: ApplicationDateFormat) => formatApplicationDate(value, locale.value, format),
    number: (value: number, format?: ApplicationNumberFormat) => formatApplicationNumber(value, locale.value, format),
    duration: (minutes: number) => formatApplicationDuration(minutes, locale.value),
    relativeTime: (value: number, unit: Intl.RelativeTimeFormatUnit) => formatApplicationRelativeTime(value, unit, locale.value),
  }
}
