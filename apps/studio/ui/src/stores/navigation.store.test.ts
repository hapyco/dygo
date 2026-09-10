import test from 'node:test'
import assert from 'node:assert/strict'
import { createPinia, setActivePinia } from 'pinia'

import { useNavigationStore } from './navigation.store.ts'
import { usePreferencesStore } from '../features/preferences/preferences.store.ts'

test('recent pages are isolated by user and logout preserves server preferences', async () => {
  const storage = memoryStorage()
  const hadWindow = 'window' in globalThis
  const previousWindow = globalThis.window
  Object.defineProperty(globalThis, 'window', { configurable: true, value: { localStorage: storage } })
  const originalFetch = globalThis.fetch
  globalThis.fetch = async () => Response.json({ data: {
    'studio.recent-pages': usePreferencesStore().userID === 7
      ? [{ path: '/customers', label: 'Customers', detail: 'Record list' }] : [],
  } })

  try {
    setActivePinia(createPinia())
    const navigation = useNavigationStore()
    const preferences = usePreferencesStore()
    navigation.setRecentUser(7)
    await preferences.startSession(7)
    navigation.rememberRecentPage({ path: '/customers', label: 'Customers', detail: 'Record list' })

    navigation.setRecentUser(8)
    await preferences.startSession(8)
    assert.deepEqual(navigation.recentPages, [])
    navigation.rememberRecentPage({ path: '/orders', label: 'Orders', detail: 'Record list' })

    navigation.setRecentUser(7)
    await preferences.startSession(7)
    assert.deepEqual(navigation.recentPages.map((page) => page.path), ['/customers'])
    navigation.setRecentUser(null)
    assert.deepEqual(navigation.recentPages, [])

    navigation.setRecentUser(7)
    await preferences.startSession(7)
    assert.deepEqual(navigation.recentPages.map((page) => page.path), ['/customers'])
    navigation.setRecentUser(8)
    await preferences.startSession(8)
    assert.deepEqual(navigation.recentPages, [])
  } finally {
    await usePreferencesStore().startSession(null)
    globalThis.fetch = originalFetch
    if (hadWindow) {
      Object.defineProperty(globalThis, 'window', { configurable: true, value: previousWindow })
    } else {
      delete (globalThis as { window?: unknown }).window
    }
  }
})

function memoryStorage(): Storage {
  const values = new Map<string, string>()
  return {
    getItem: (key) => values.get(key) ?? null,
    setItem: (key, value) => values.set(key, String(value)),
    removeItem: (key) => values.delete(key),
    clear: () => values.clear(),
    key: (index) => [...values.keys()][index] ?? null,
    get length() { return values.size },
  }
}

test('a page visited during hydration keeps server history', async () => {
  const original = globalThis.fetch
  let resolve!: (response: Response) => void
  globalThis.fetch = () => new Promise<Response>(done => { resolve = done })
  setActivePinia(createPinia())
  const navigation = useNavigationStore()
  try {
    navigation.setRecentUser(7)
    const remember = navigation.rememberRecentPage({ path: '/current', label: 'Current', detail: '' })
    resolve(Response.json({ data: { 'studio.recent-pages': [{ path: '/previous', label: 'Previous', detail: '' }] } }))
    await remember
    assert.deepEqual(navigation.recentPages.map(page => page.path), ['/current', '/previous'])
  } finally {
    navigation.setRecentUser(null)
    globalThis.fetch = original
  }
})

test('open tabs stay in the browser tab and survive reload without wiping', () => {
  const session = memoryStorage()
  const hadWindow = 'window' in globalThis
  const previousWindow = globalThis.window
  Object.defineProperty(globalThis, 'window', { configurable: true, value: { sessionStorage: session, localStorage: memoryStorage() } })
  const originalFetch = globalThis.fetch
  const writes: string[] = []
  globalThis.fetch = async (_input, init) => {
    if (init?.method === 'PUT') writes.push(String(_input))
    return Response.json({ data: { 'studio.open-tabs': [{ path: '/server', fullPath: '/server', label: 'Server' }] } })
  }

  try {
    setActivePinia(createPinia())
    const first = useNavigationStore()
    first.setRecentUser(7)
    first.syncTab({ path: '/constraint', fullPath: '/constraint', label: 'Constraint' })
    first.syncTab({ path: '/constraint/studio.preference.preference-user-key-key', fullPath: '/constraint/studio.preference.preference-user-key-key', label: 'Constraint / record' })
    first.closeTab('/constraint', '/constraint/studio.preference.preference-user-key-key')
    assert.deepEqual(first.openTabs.map(tab => tab.path), ['/constraint/studio.preference.preference-user-key-key'])
    assert.equal(writes.some(url => url.includes('studio.open-tabs')), false)

    setActivePinia(createPinia())
    const reloaded = useNavigationStore()
    reloaded.setRecentUser(7)
    reloaded.syncTab({ path: '/constraint/studio.preference.preference-user-key-key', fullPath: '/constraint/studio.preference.preference-user-key-key', label: 'Constraint / record' })
    assert.deepEqual(reloaded.openTabs.map(tab => tab.path), ['/constraint/studio.preference.preference-user-key-key'])

    const otherSession = memoryStorage()
    Object.defineProperty(globalThis, 'window', { configurable: true, value: { sessionStorage: otherSession, localStorage: memoryStorage() } })
    setActivePinia(createPinia())
    const otherTab = useNavigationStore()
    otherTab.setRecentUser(7)
    otherTab.syncTab({ path: '/user', fullPath: '/user', label: 'User' })
    assert.deepEqual(otherTab.openTabs.map(tab => tab.path), ['/user'])
  } finally {
    globalThis.fetch = originalFetch
    if (hadWindow) {
      Object.defineProperty(globalThis, 'window', { configurable: true, value: previousWindow })
    } else {
      delete (globalThis as { window?: unknown }).window
    }
  }
})
