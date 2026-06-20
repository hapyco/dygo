import test from 'node:test'
import assert from 'node:assert/strict'
import { createPinia, setActivePinia } from 'pinia'

import { useDialogStore } from './dialogs.store.ts'

test('dialog store defaults dismissible dialogs', async () => {
  setActivePinia(createPinia())
  const store = useDialogStore()

  const result = store.open({ title: 'Saved' })
  const dialog = store.topDialog
  assert.equal(dialog?.type, 'neutral')
  assert.equal(dialog?.dismissible, true)
  assert.deepEqual(dialog?.actions, [{ key: 'ok', label: 'OK', variant: 'primary' }])

  store.selectAction(dialog?.id ?? 0, 'ok')
  assert.equal(await result, 'ok')
})

test('dialog store rejects non-dismissible dialogs without actions', () => {
  setActivePinia(createPinia())
  const store = useDialogStore()

  assert.throws(
    () => store.open({ title: 'Required', dismissible: false }),
    /requires an action/,
  )
})

test('dialog store rejects duplicate action keys', () => {
  setActivePinia(createPinia())
  const store = useDialogStore()

  assert.throws(
    () => store.open({
      title: 'Choose',
      actions: [
        { key: 'same', label: 'First' },
        { key: 'same', label: 'Second' },
      ],
    }),
    /duplicate dialog action key/,
  )
})

test('dialog store resolves the top dialog independently', async () => {
  setActivePinia(createPinia())
  const store = useDialogStore()

  const first = store.open({ title: 'First' })
  const firstID = store.topDialog?.id ?? 0
  const second = store.open({ title: 'Second' })
  const secondID = store.topDialog?.id ?? 0

  assert.equal(store.topDialog?.title, 'Second')
  store.selectAction(secondID, 'ok')
  assert.equal(await second, 'ok')

  assert.equal(store.topDialog?.title, 'First')
  store.dismissTop()
  assert.equal(await first, null)
  assert.equal(store.stack.length, 0)
  assert.notEqual(firstID, secondID)
})
