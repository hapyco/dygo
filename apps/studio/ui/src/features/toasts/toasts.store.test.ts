import test from 'node:test'
import assert from 'node:assert/strict'
import { createPinia, setActivePinia } from 'pinia'

import { useToastStore } from './toasts.store.ts'

test('toast store defaults and dismisses toasts', () => {
  setActivePinia(createPinia())
  const store = useToastStore()

  store.show({ title: 'Saved' })
  assert.equal(store.toasts.length, 1)
  assert.equal(store.toasts[0].type, 'info')
  assert.equal(store.toasts[0].duration, 4000)

  store.dismiss(store.toasts[0].id)
  assert.equal(store.toasts.length, 0)
})

test('toast store validates title and duration', () => {
  setActivePinia(createPinia())
  const store = useToastStore()

  assert.throws(() => store.show({ title: '' }), /toast title is required/)
  assert.throws(() => store.show({ title: 'Bad', duration: -1 }), /duration/)
})

test('toast store keeps zero duration sticky', () => {
  setActivePinia(createPinia())
  const store = useToastStore()

  store.show({ title: 'Stay open', duration: 0 })

  assert.equal(store.toasts[0].duration, 0)
})
