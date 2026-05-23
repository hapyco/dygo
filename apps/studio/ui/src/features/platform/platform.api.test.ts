import test from 'node:test'
import assert from 'node:assert/strict'

import { getPlatformConfig } from './platform.api.ts'

test('getPlatformConfig reads backend record list policy', async (t) => {
  const originalFetch = globalThis.fetch
  t.after(() => {
    globalThis.fetch = originalFetch
  })

  globalThis.fetch = (async (input, init) => {
    assert.equal(input, '/api/v1/platform')
    assert.equal(init?.credentials, 'include')
    return new Response(JSON.stringify({
      data: {
        'record-list': {
          'default-limit': 20,
          'max-limit': 2500,
          'page-sizes': [20, 100, 500, 2500],
        },
      },
    }))
  }) as typeof fetch

  const config = await getPlatformConfig()
  assert.deepEqual(config['record-list'], {
    'default-limit': 20,
    'max-limit': 2500,
    'page-sizes': [20, 100, 500, 2500],
  })
})
