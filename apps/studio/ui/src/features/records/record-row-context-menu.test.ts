import assert from 'node:assert/strict'
import test from 'node:test'

import { recordRowContextMenuItems } from './record-row-context-menu.ts'

test('recordRowContextMenuItems disables actions without a Record name', () => {
  const items = recordRowContextMenuItems({ id: 1 })
  const open = items.find((item) => item.type === 'item' && item.key === 'open')
  assert.equal(open && open.type === 'item' ? open.disabled : undefined, true)

  const named = recordRowContextMenuItems({ id: 1, name: 'lead-1' })
  const namedOpen = named.find((item) => item.type === 'item' && item.key === 'open')
  assert.equal(namedOpen && namedOpen.type === 'item' ? namedOpen.disabled : undefined, false)
})
