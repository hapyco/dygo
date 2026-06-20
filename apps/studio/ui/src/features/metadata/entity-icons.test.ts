import test from 'node:test'
import assert from 'node:assert/strict'
import { Box, Settings2 } from '@lucide/vue'

import { iconForEntity } from './entity-icons.ts'

test('iconForEntity resolves metadata icon names without bundling all Lucide icons', () => {
  assert.equal(iconForEntity('settings-2'), Settings2)
  assert.equal(iconForEntity('Settings2'), Settings2)
  assert.equal(iconForEntity('missing-icon'), Box)
})
