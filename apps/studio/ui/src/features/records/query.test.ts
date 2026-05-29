import test from 'node:test'
import assert from 'node:assert/strict'

import {
  buildRecordListRouteQuery,
  buildRecordListQuery,
  canonicalizeRecordListRouteQuery,
  isAllowedRecordPageSize,
  parseRecordListRouteQuery,
  recordListRouteQueriesEqual,
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

test('canonicalizeRecordListRouteQuery drops invalid params and normalizes values', () => {
  const result = canonicalizeRecordListRouteQuery({
    sort: '-created-at,name',
    offset: '40',
    'status:eq': ' Open ',
    'status:unknown': 'ignored',
    'archived-at:empty': 'ignored',
    'deleted-at:empty': null,
    'name:contains': ' ABC ',
    'amount:gte': '',
    'missing:eq': 'value',
    'bad:key:extra': 'value',
  }, {
    sortableFields: ['created-at', 'name'],
    filterFields: [
      { field: 'name', operators: [{ key: 'contains', arity: 'one' }] },
      { field: 'status', operators: [{ key: 'eq', arity: 'one' }] },
      { field: 'archived-at', operators: [{ key: 'empty', arity: 'none' }] },
      { field: 'deleted-at', operators: [{ key: 'empty', arity: 'none' }] },
      { field: 'amount', operators: [{ key: 'gte', arity: 'one' }] },
    ],
  })

  assert.equal(result.changed, true)
  assert.deepEqual(result.state, {
    sort: { key: 'created-at', direction: 'desc' },
    filters: [
      { field: 'status', operator: 'eq', value: 'Open' },
      { field: 'archived-at', operator: 'empty' },
      { field: 'deleted-at', operator: 'empty' },
      { field: 'name', operator: 'contains', value: 'ABC' },
    ],
  })
  assert.deepEqual(result.query, {
    sort: '-created-at',
    'status:eq': 'Open',
    'archived-at:empty': '',
    'deleted-at:empty': '',
    'name:contains': 'ABC',
  })
})

test('canonicalizeRecordListRouteQuery keeps already canonical URLs unchanged', () => {
  const query = {
    sort: 'name',
    'enabled:eq': 'true',
    'archived-at:empty': '',
  }

  const result = canonicalizeRecordListRouteQuery(query, {
    sortableFields: ['name'],
    filterFields: [
      { field: 'enabled', operators: [{ key: 'eq', arity: 'one' }] },
      { field: 'archived-at', operators: [{ key: 'empty', arity: 'none' }] },
    ],
  })

  assert.equal(result.changed, false)
  assert.equal(recordListRouteQueriesEqual(query, result.query), true)
  assert.deepEqual(result.state, {
    sort: { key: 'name', direction: 'asc' },
    filters: [
      { field: 'enabled', operator: 'eq', value: 'true' },
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
