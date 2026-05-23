import test from 'node:test'
import assert from 'node:assert/strict'

import {
  buildRecordListQuery,
  isAllowedRecordPageSize,
} from './query.ts'

test('buildRecordListQuery serializes pagination, sort, and filters', () => {
  const query = buildRecordListQuery({
    limit: 25,
    offset: 5,
    sort: { key: 'created-at', direction: 'desc' },
    filters: {
      status: 'Open',
      enabled: 'true',
    },
  })

  assert.equal(query.get('limit'), '25')
  assert.equal(query.get('offset'), '5')
  assert.equal(query.get('sort'), '-created-at')
  assert.equal(query.get('status'), 'Open')
  assert.equal(query.get('enabled'), 'true')
})

test('isAllowedRecordPageSize checks backend-provided page sizes', () => {
  const pageSizes = [20, 100, 500, 2500]

  assert.equal(isAllowedRecordPageSize(20, pageSizes), true)
  assert.equal(isAllowedRecordPageSize(2500, pageSizes), true)
  assert.equal(isAllowedRecordPageSize(50, pageSizes), false)
})
