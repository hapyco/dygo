import test from 'node:test'
import assert from 'node:assert/strict'

import { formatRecordDisplayValue } from './values.ts'

test('formatRecordDisplayValue renders datetime values in browser-local ISO-like format', () => {
  const localDate = new Date(2026, 4, 24, 3, 29, 14)
  const apiValue = localDate.toISOString()

  assert.equal(formatRecordDisplayValue(apiValue, 'datetime'), '2026-05-24 03:29:14')
})

test('formatRecordDisplayValue falls back safely for invalid datetime values', () => {
  assert.equal(formatRecordDisplayValue('not a date', 'datetime'), 'not a date')
  assert.equal(formatRecordDisplayValue('', 'datetime'), '-')
  assert.equal(formatRecordDisplayValue(null, 'datetime'), '-')
})

test('formatRecordDisplayValue keeps fallback rendering for other display types', () => {
  assert.equal(formatRecordDisplayValue(true, 'boolean'), 'true')
  assert.equal(formatRecordDisplayValue({ ok: true }, 'json'), '{"ok":true}')
})
