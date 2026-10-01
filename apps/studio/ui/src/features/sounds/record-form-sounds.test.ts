import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

test('record form mutations bind save, error, and delete sounds', () => {
  const source = readFileSync(new URL('../records/record-form.query.ts', import.meta.url), 'utf8')
  const saveHandlers = source.match(/studioMutationSoundHandlers<[^>]+>\('save'/g) ?? []
  const deleteHandlers = source.match(/studioMutationSoundHandlers<[^>]+>\('delete'/g) ?? []

  assert.equal(saveHandlers.length, 4)
  assert.equal(deleteHandlers.length, 1)
  assert.match(source, /studioSounds\.error\(\)/)
})
