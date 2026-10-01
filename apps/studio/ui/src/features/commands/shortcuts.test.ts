import assert from 'node:assert/strict'
import test from 'node:test'
import { computed, effectScope, nextTick, ref } from 'vue'
import { bindingConflicts, executeCommand, matchShortcut, nextAvailableCommandId, shortcutLabel, ariaShortcut, type StudioCommand } from './shortcuts.ts'
import { pageCommands, usePageCommands } from './context.ts'

const save: StudioCommand = { id: 'record:save', label: 'Save', run() {} }
function key(overrides = {}) {
  return { key: 's', metaKey: true, ctrlKey: false, shiftKey: false, altKey: false, repeat: false, isComposing: false, defaultPrevented: false, getModifierState: () => false, ...overrides }
}
test('exact platform modifiers, input policy, handled events and overlays', () => {
  assert.equal(matchShortcut(key(), [save], true, false, true)?.id, save.id)
  assert.equal(matchShortcut(key({ metaKey: false, ctrlKey: true }), [save], true, false, false)?.id, save.id)
  for (const override of [{ metaKey: false }, { ctrlKey: true }, { shiftKey: true }, { altKey: true }, { repeat: true }, { isComposing: true }, { defaultPrevented: true }, { getModifierState: () => true }]) {
    assert.equal(matchShortcut(key(override), [save], false, false, true), undefined, JSON.stringify(override))
  }
  assert.equal(matchShortcut(key(), [save], true, true, true), undefined)
  const create = { ...save, id: 'records-new' }
  assert.equal(matchShortcut(key({ key: 'Enter' }), [create], true, false, true), undefined)
  assert.equal(matchShortcut(key({ key: 'Enter' }), [create], false, false, true)?.id, create.id)
  assert.equal(matchShortcut(key({ key: '/' }), [{ ...save, id: 'app:shortcuts' }], true, false, true)?.id, 'app:shortcuts')
  assert.equal(shortcutLabel('s', false), 'Ctrl+S')
  assert.equal(ariaShortcut('s', true), 'Meta+s')
})
test('collisions never resolve to an arbitrary command; disabled Save is reserved', async () => {
  const conflict = { ...save, id: 'other', shortcut: 'S' }
  assert.deepEqual(bindingConflicts([save, conflict]), [['s', ['record:save', 'other']]])
  assert.equal(matchShortcut(key(), [save, conflict], false, false, true), undefined)
  let calls = 0
  const disabled = { ...save, disabledReason: 'No changes', run: () => { calls++ } }
  assert.equal(matchShortcut(key(), [disabled], true, false, true)?.consumeDisabled, true)
  await executeCommand(disabled)
  assert.equal(calls, 0)
})
test('all invocation paths share disabled and duplicate-submit checks', async () => {
  let finish!: () => void
  let calls = 0
  const command = { ...save, run: () => { calls++; return new Promise<void>(resolve => { finish = resolve }) } }
  const first = executeCommand(command)
  await executeCommand(command)
  assert.equal(calls, 1)
  finish(); await first
  command.run = async () => { calls++ }
  await executeCommand(command)
  assert.equal(calls, 2)
})
test('palette arrow navigation skips unavailable commands and wraps', () => {
  const open = { id: 'open', label: 'Open', run() {} }
  const blocked = { id: 'blocked', label: 'Blocked', disabledReason: 'Unavailable', run() {} }
  const next = { id: 'next', label: 'Next', run() {} }
  const items = [open, blocked, next]
  assert.equal(nextAvailableCommandId(items, open.id, 1), next.id)
  assert.equal(nextAvailableCommandId(items, next.id, 1), open.id)
  assert.equal(nextAvailableCommandId(items, next.id, -1), open.id)
  assert.equal(nextAvailableCommandId(items, blocked.id, 1), open.id)
  assert.equal(nextAvailableCommandId(items, '', -1), next.id)
  assert.equal(nextAvailableCommandId([blocked], blocked.id, 1), '')
})
test('background page updates cannot reclaim commands; disposal restores the parent', async () => {
  const label = ref('List')
  const parent = effectScope()
  parent.run(() => usePageCommands(computed(() => [{ ...save, id: 'records-new', label: label.value }])))
  const form = effectScope()
  form.run(() => usePageCommands(computed(() => [save])))
  label.value = 'Changed list'; await nextTick()
  assert.equal(pageCommands.value[0]?.id, 'record:save')
  form.stop()
  assert.equal(pageCommands.value[0]?.label, 'Changed list')
  parent.stop()
  assert.deepEqual(pageCommands.value, [])
})
