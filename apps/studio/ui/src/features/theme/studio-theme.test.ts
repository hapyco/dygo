import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

import {
  applyStudioTheme,
  getStudioThemePreference,
  installStudioTheme,
  isStudioThemePreference,
  readStudioThemePreference,
  resolveStudioTheme,
  setStudioThemePreference,
  studioThemeOptions,
  studioThemeStorageKey,
  syncStudioTheme,
} from './studio-theme.ts'

test('readStudioThemePreference defaults to system and accepts stored values', () => {
  assert.equal(readStudioThemePreference(null), 'system')
  assert.equal(readStudioThemePreference('light'), 'light')
  assert.equal(readStudioThemePreference('dark'), 'dark')
  assert.equal(readStudioThemePreference('system'), 'system')
  assert.equal(readStudioThemePreference('sepia'), 'system')
})

test('resolveStudioTheme follows explicit preference and system color scheme', () => {
  assert.equal(resolveStudioTheme('light', true), 'light')
  assert.equal(resolveStudioTheme('dark', false), 'dark')
  assert.equal(resolveStudioTheme('system', true), 'dark')
  assert.equal(resolveStudioTheme('system', false), 'light')
})

test('studio theme options cover light, dark, and system', () => {
  assert.deepEqual(studioThemeOptions.map((option) => option?.value).filter(Boolean), ['light', 'dark', 'system'])
})

test('theme preferences remain safe when browser storage is unavailable', () => {
  installWindow({
    localStorage: {
      getItem: () => {
        throw new Error('storage unavailable')
      },
      setItem: () => {
        throw new Error('storage unavailable')
      },
    },
    matchMedia: () => ({
      matches: true,
      addEventListener: () => {
        throw new Error('matchMedia unavailable')
      },
    }),
  })

  assert.equal(getStudioThemePreference(), 'system')
  assert.doesNotThrow(() => setStudioThemePreference('dark'))
  assert.doesNotThrow(() => installStudioTheme())
})

test('setStudioThemePreference persists and applies the resolved theme', () => {
  const storage = memoryStorage()
  const root = themeRoot()
  installWindow({
    localStorage: storage,
    matchMedia: () => ({ matches: false, addEventListener() {} }),
  })

  assert.equal(setStudioThemePreference('dark', root), 'dark')
  assert.equal(storage.getItem(studioThemeStorageKey), 'dark')
  assert.equal(root.dataset.theme, 'dark')

  storage.setItem(studioThemeStorageKey, 'system')
  assert.equal(syncStudioTheme(root), 'light')
  assert.equal(root.dataset.theme, 'light')
})

test('applyStudioTheme writes data-theme on the document root', () => {
  const root = themeRoot()
  applyStudioTheme('dark', root)
  assert.equal(root.dataset.theme, 'dark')
  applyStudioTheme('light', root)
  assert.equal(root.dataset.theme, 'light')
})

test('dark theme tokens override the light palette', () => {
  const source = readFileSync(new URL('../../styles/base.css', import.meta.url), 'utf8')
  assert.match(source, /:root\[data-theme='dark'\]/)
  assert.match(source, /--studio-accent-contrast/)
  assert.match(source, /--studio-overlay/)
})

test('teleported Studio panels keep their component styles', () => {
  for (const component of ['../dialogs/DialogHost.vue', '../notifications/NotificationMenu.vue']) {
    const source = readFileSync(new URL(component, import.meta.url), 'utf8')
    assert.match(source, /<style>/)
    assert.doesNotMatch(source, /<style scoped>/)
  }
})

test('index.html bootstraps the stored theme before Vue mounts', () => {
  const source = readFileSync(new URL('../../../index.html', import.meta.url), 'utf8')
  assert.match(source, /studio:theme/)
  assert.match(source, /dataset\.theme/)
  assert.match(source, /prefers-color-scheme: dark/)
})

test('isStudioThemePreference rejects unknown values', () => {
  assert.equal(isStudioThemePreference('light'), true)
  assert.equal(isStudioThemePreference('auto'), false)
  assert.equal(isStudioThemePreference(1), false)
})

function installWindow(overrides: Record<string, unknown>) {
  Object.defineProperty(globalThis, 'window', {
    configurable: true,
    value: overrides,
  })
}

function memoryStorage() {
  const values = new Map<string, string>()
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => {
      values.set(key, value)
    },
  }
}

function themeRoot() {
  return { dataset: {} as Record<string, string> } as unknown as HTMLElement
}
