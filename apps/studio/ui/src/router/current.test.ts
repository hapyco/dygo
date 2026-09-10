import assert from 'node:assert/strict'
import test from 'node:test'

import { studioPathIsEntity, studioPinIsCurrent, studioPathsEqual } from './current.ts'

test('entity navigation stays current on Record URLs', () => {
  const record = '/constraint/studio.preference.preference-user-key-key'
  assert.equal(studioPathIsEntity(record, 'constraint'), true)
  assert.equal(studioPathIsEntity('/constraint', 'constraint'), true)
  assert.equal(studioPathIsEntity('/constraint/new', 'constraint'), true)
  assert.equal(studioPathIsEntity(record, 'preference'), false)
  assert.equal(studioPathIsEntity('/constraint-type/x', 'constraint'), false)
})

test('entity pins stay current on child Record paths', () => {
  const record = '/constraint/studio.preference.preference-user-key-key'
  assert.equal(studioPinIsCurrent('entity', '/constraint', record), true)
  assert.equal(studioPinIsCurrent('entity', '/constraint', '/constraint'), true)
  assert.equal(studioPinIsCurrent('record', record, record), true)
  assert.equal(studioPinIsCurrent('record', record, '/constraint'), false)
  assert.equal(studioPinIsCurrent('page', '/', record), false)
  assert.equal(studioPathsEqual('/customers/CUS-1', '/customers/CUS-1'), true)
})
