import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

import {
  isStudioSoundEnabled,
  readStudioSoundEnabled,
  setStudioSoundEnabled,
  studioSoundPaths,
  studioSounds,
} from './studio-sounds.ts'

test('readStudioSoundEnabled defaults to enabled', () => {
  assert.equal(readStudioSoundEnabled(null, false), true)
  assert.equal(readStudioSoundEnabled('true', false), true)
})

test('readStudioSoundEnabled respects explicit disable and reduced motion', () => {
  assert.equal(readStudioSoundEnabled('false', false), false)
  assert.equal(readStudioSoundEnabled(null, true), false)
  assert.equal(readStudioSoundEnabled('true', true), false)
})

test('studio sound paths include save, error, and delete', () => {
  assert.equal(studioSoundPaths.save, '/sounds/save.mp3')
  assert.equal(studioSoundPaths.error, '/sounds/error.mp3')
  assert.equal(studioSoundPaths.delete, '/sounds/delete.mp3')
})

test('bundled studio sound files exist', () => {
  const publicRoot = new URL('../../../public/', import.meta.url)

  for (const path of Object.values(studioSoundPaths)) {
    const filePath = new URL(path.replace(/^\//, ''), publicRoot)
    assert.doesNotThrow(() => readFileSync(filePath))
  }
})

test('sound preferences remain safe when browser storage is unavailable', () => {
  installWindow({
    localStorage: {
      getItem: () => { throw new Error('storage unavailable') },
      setItem: () => { throw new Error('storage unavailable') },
    },
    matchMedia: () => ({ matches: false }),
  })

  assert.equal(isStudioSoundEnabled(), true)
  assert.doesNotThrow(() => setStudioSoundEnabled(false))
})

test('sound playback is best-effort and respects the preference', () => {
  let playCount = 0
  installWindow({
    localStorage: memoryStorage(),
    matchMedia: () => ({ matches: false }),
    Audio: class {
      preload = ''
      play() {
        playCount += 1
        return Promise.reject(new Error('playback blocked'))
      }
    },
  })

  setStudioSoundEnabled(true)
  assert.doesNotThrow(() => studioSounds.save())
  assert.equal(playCount, 1)

  setStudioSoundEnabled(false)
  studioSounds.error()
  assert.equal(playCount, 1)
})

test('sound playback contains synchronous browser failures', () => {
  installWindow({
    localStorage: memoryStorage(),
    matchMedia: () => ({ matches: false }),
    Audio: class {
      constructor() {
        throw new Error('audio unavailable')
      }
    },
  })

  setStudioSoundEnabled(true)
  assert.doesNotThrow(() => studioSounds.delete())
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
    setItem: (key: string, value: string) => values.set(key, value),
  }
}
