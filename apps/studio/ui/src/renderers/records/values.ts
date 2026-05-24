export function formatRecordDisplayValue(value: unknown, display: string): string {
  if (display === 'datetime') {
    return formatRecordDateTimeValue(value)
  }

  return fallbackRecordDisplayValue(value)
}

function formatRecordDateTimeValue(value: unknown): string {
  if (value === null || value === undefined || value === '') {
    return '-'
  }

  if (typeof value !== 'string' && typeof value !== 'number' && !(value instanceof Date)) {
    return fallbackRecordDisplayValue(value)
  }

  const date = value instanceof Date ? value : new Date(value)
  if (Number.isNaN(date.getTime())) {
    return fallbackRecordDisplayValue(value)
  }

  return [
    date.getFullYear(),
    padDatePart(date.getMonth() + 1),
    padDatePart(date.getDate()),
  ].join('-') + ' ' + [
    padDatePart(date.getHours()),
    padDatePart(date.getMinutes()),
    padDatePart(date.getSeconds()),
  ].join(':')
}

function fallbackRecordDisplayValue(value: unknown): string {
  if (value === null || value === undefined || value === '') {
    return '-'
  }

  if (typeof value === 'string') {
    return value
  }

  if (typeof value === 'number' || typeof value === 'bigint' || typeof value === 'boolean') {
    return String(value)
  }

  try {
    return JSON.stringify(value)
  } catch {
    return String(value)
  }
}

function padDatePart(value: number): string {
  return String(value).padStart(2, '0')
}
