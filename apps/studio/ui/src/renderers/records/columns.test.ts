import test from 'node:test'
import assert from 'node:assert/strict'

import type { MetadataField } from '../../features/metadata/metadata.api'
import { buildRecordListColumns } from './columns.ts'

test('buildRecordListColumns uses metadata display hints and listability', () => {
  const columns = buildRecordListColumns([
    metadataField({ name: 'email', label: 'Email', listable: true, display: 'email' }),
    metadataField({ name: 'password', label: 'Password', listable: true, writeOnly: true, display: 'hidden' }),
    metadataField({ name: 'notes', label: 'Notes', listable: false, display: 'text' }),
  ], systemFields())

  assert.deepEqual(columns.map((column) => column.key), ['name', 'email', 'created-at', 'updated-at'])
  assert.equal(columns.find((column) => column.key === 'email')?.cellType, 'email')
  assert.equal(columns.find((column) => column.key === 'created-at')?.formatValue?.('bad datetime'), 'bad datetime')
})

test('buildRecordListColumns keeps the system name column authoritative', () => {
  const columns = buildRecordListColumns([
    metadataField({ name: 'name', label: 'Custom Name', listable: true, display: 'text' }),
  ], systemFields())

  assert.equal(columns.filter((column) => column.key === 'name').length, 1)
  assert.equal(columns[0].source, 'name')
  assert.equal(columns[0].label, 'ID')
})

test('buildRecordListColumns uses system field metadata when provided', () => {
  const columns = buildRecordListColumns([
    metadataField({ name: 'email', label: 'Email', listable: true, display: 'email' }),
  ], [
    metadataField({ name: 'id', label: 'ID', listable: true, display: 'number' }),
    metadataField({ name: 'name', label: 'Record Name', listable: true, display: 'text' }),
    metadataField({ name: 'updated-at', label: 'Updated', listable: true, display: 'datetime' }),
  ])

  assert.deepEqual(columns.map((column) => column.key), ['name', 'email', 'updated-at'])
  assert.equal(columns[0].label, 'ID')
  assert.equal(columns.find((column) => column.key === 'updated-at')?.cellType, 'datetime')
  assert.equal(columns.find((column) => column.key === 'updated-at')?.formatValue?.(new Date(2026, 4, 24, 3, 29, 14)), '2026-05-24 03:29:14')
})

function metadataField(overrides: {
  name: string
  label: string
  listable: boolean
  writeOnly?: boolean
  display: string
}): MetadataField {
  return {
    name: overrides.name,
    label: overrides.label,
    type: 'text',
    required: false,
    unique: false,
    index: false,
    stored: true,
    'write-only': overrides.writeOnly ?? false,
    listable: overrides.listable,
    'name-renderable': true,
    'value-kind': 'string',
    studio: {
      editor: 'text',
      display: overrides.display,
    },
    position: 1,
  }
}

function systemFields(): MetadataField[] {
  return [
    metadataField({ name: 'id', label: 'ID', listable: false, display: 'number' }),
    metadataField({ name: 'name', label: 'Name', listable: true, display: 'text' }),
    metadataField({ name: 'created-at', label: 'Created At', listable: true, display: 'datetime' }),
    metadataField({ name: 'updated-at', label: 'Updated At', listable: true, display: 'datetime' }),
  ]
}
