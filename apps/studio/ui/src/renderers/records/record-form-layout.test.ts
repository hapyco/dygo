import assert from 'node:assert/strict'
import test from 'node:test'
import { resolveFormLayout } from './record-form-layout.ts'

test('flat forms keep all visible fields without a tab strip', () => {
  const result = resolveFormLayout(null, [{ name: 'name' }, { name: 'title' }], 'Customer')
  assert.equal(result.showTabStrip, false)
  assert.deepEqual(result.form.tabs[0]?.items.map((item) => item.name), ['name', 'title'])
})

test('one explicit tab remains visible and includes the manual Record ID once', () => {
  const authored = { tabs: [{ key: 'general', label: 'General', items: [{ kind: 'field' as const, name: 'title' }] }] }
  const result = resolveFormLayout(authored, [{ name: 'name' }, { name: 'title' }], 'Customer')
  assert.equal(result.showTabStrip, true)
  assert.deepEqual(result.form.tabs[0]?.items.map((item) => item.name), ['name', 'title'])
  assert.deepEqual(authored.tabs[0]?.items.map((item) => item.name), ['title'])
})

test('fields assigned to another tab retain their position', () => {
  const authored = { tabs: [
    { key: 'general', label: 'General', items: [{ kind: 'field' as const, name: 'title' }] },
    { key: 'details', label: 'Details', items: [{ kind: 'section' as const, label: 'Details' }, { kind: 'field' as const, name: 'note' }] },
  ] }
  const result = resolveFormLayout(authored, [{ name: 'name' }, { name: 'title' }, { name: 'note' }], 'Customer')
  assert.deepEqual(result.form.tabs[0]?.items.map((item) => item.name), ['name', 'title'])
  assert.deepEqual(result.form.tabs[1], authored.tabs[1])
})
