import test from 'node:test'
import assert from 'node:assert/strict'

import {
  buildRecordListRouteQuery,
  buildRecordListQuery,
  isAllowedRecordPageSize,
  parseRecordListRouteQuery,
} from './query.ts'

test('buildRecordListQuery serializes pagination, sort, and filters', () => {
  const query = buildRecordListQuery({
    limit: 25,
    offset: 5,
    sort: { key: 'created-at', direction: 'desc' },
    filters: [
      { field: 'status', operator: 'eq', value: 'Open' },
      { field: 'enabled', operator: 'eq', value: 'true' },
      { field: 'archived-at', operator: 'empty' },
    ],
  })

  assert.equal(query.get('limit'), '25')
  assert.equal(query.get('offset'), '5')
  assert.equal(query.get('sort'), '-created-at')
  assert.equal(query.get('status:eq'), 'Open')
  assert.equal(query.get('enabled:eq'), 'true')
  assert.equal(query.get('archived-at:empty'), '')
})

test('buildRecordListRouteQuery serializes shareable list state', () => {
  const query = buildRecordListRouteQuery({
    sort: { key: 'created-at', direction: 'desc' },
    filters: [
      { field: 'status', operator: 'eq', value: 'Open' },
      { field: 'amount', operator: 'gte', value: '10' },
      { field: 'amount', operator: 'lte', value: '100' },
      { field: 'archived-at', operator: 'empty' },
    ],
  })

  assert.deepEqual(query, {
    sort: '-created-at',
    'status:eq': 'Open',
    'amount:gte': '10',
    'amount:lte': '100',
    'archived-at:empty': '',
  })
})

test('parseRecordListRouteQuery reads filters and single-column sort from the URL', () => {
  const state = parseRecordListRouteQuery({
    sort: '-created-at,name',
    'status:eq': 'Open',
    'amount:gte': '10',
    'archived-at:empty': '',
    ignored: 'value',
  })

  assert.deepEqual(state, {
    sort: { key: 'created-at', direction: 'desc' },
    filters: [
      { field: 'status', operator: 'eq', value: 'Open' },
      { field: 'amount', operator: 'gte', value: '10' },
      { field: 'archived-at', operator: 'empty' },
    ],
  })
})

test('isAllowedRecordPageSize checks backend-provided page sizes', () => {
  const pageSizes = [20, 100, 500, 2500]

  assert.equal(isAllowedRecordPageSize(20, pageSizes), true)
  assert.equal(isAllowedRecordPageSize(2500, pageSizes), true)
  assert.equal(isAllowedRecordPageSize(50, pageSizes), false)
})
