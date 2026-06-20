import test from 'node:test'
import assert from 'node:assert/strict'

import {
  ApiClientError,
  apiRequest,
  setAPIDialogHandler,
  type ApiErrorEnvelope,
  type DataEnvelope,
} from './client.ts'

class TestApiError extends ApiClientError {
  constructor(code: string, message: string, details?: Record<string, unknown>) {
    super('TestApiError', code, message, details)
  }
}

test('apiRequest applies credentials and returns successful envelopes', async (t) => {
  const originalFetch = globalThis.fetch
  t.after(() => {
    globalThis.fetch = originalFetch
  })

  let observedCredentials: RequestCredentials | undefined
  globalThis.fetch = (async (_input, init) => {
    observedCredentials = init?.credentials
    return new Response(JSON.stringify({ data: { ok: true } }), { status: 200 })
  }) as typeof fetch

  const payload = await apiRequest<DataEnvelope<{ ok: boolean }>, TestApiError>('/api/test', { method: 'GET' }, requestOptions())

  assert.deepEqual(payload.data, { ok: true })
  assert.equal(observedCredentials, 'include')
})

test('apiRequest emits successful response dialogs', async (t) => {
  const originalFetch = globalThis.fetch
  t.after(() => {
    globalThis.fetch = originalFetch
    setAPIDialogHandler(null)
  })

  let observedTitle = ''
  setAPIDialogHandler((dialog) => {
    observedTitle = dialog.title
  })
  globalThis.fetch = (async () => new Response(JSON.stringify({
    data: { ok: true },
    dialog: { title: 'Saved' },
  }), { status: 200 })) as typeof fetch

  await apiRequest<DataEnvelope<{ ok: boolean }>, TestApiError>('/api/test', { method: 'GET' }, requestOptions())

  assert.equal(observedTitle, 'Saved')
})

test('apiRequest maps error envelopes through the domain error class', async (t) => {
  const originalFetch = globalThis.fetch
  t.after(() => {
    globalThis.fetch = originalFetch
  })

  globalThis.fetch = (async () => new Response(JSON.stringify({
    error: {
      code: 'forbidden',
      message: 'permission denied',
      details: { entity: 'user' },
    },
  }), { status: 403 })) as typeof fetch

  await assert.rejects(
    apiRequest<DataEnvelope<unknown>, TestApiError>('/api/test', { method: 'GET' }, requestOptions()),
    (error) => {
      assert.equal(error instanceof TestApiError, true)
      assert.equal((error as TestApiError).code, 'forbidden')
      assert.deepEqual((error as TestApiError).details, { entity: 'user' })
      assert.equal((error as Error).message, 'mapped: permission denied')
      return true
    },
  )
})

test('apiRequest emits error response dialogs before throwing', async (t) => {
  const originalFetch = globalThis.fetch
  t.after(() => {
    globalThis.fetch = originalFetch
    setAPIDialogHandler(null)
  })

  let observedTitle = ''
  setAPIDialogHandler((dialog) => {
    observedTitle = dialog.title
  })
  globalThis.fetch = (async () => new Response(JSON.stringify({
    error: {
      code: 'forbidden',
      message: 'permission denied',
      dialog: { title: 'Access denied' },
    },
  }), { status: 403 })) as typeof fetch

  await assert.rejects(
    apiRequest<DataEnvelope<unknown>, TestApiError>('/api/test', { method: 'GET' }, requestOptions()),
    TestApiError,
  )
  assert.equal(observedTitle, 'Access denied')
})

test('apiRequest reports invalid JSON with the domain error class', async (t) => {
  const originalFetch = globalThis.fetch
  t.after(() => {
    globalThis.fetch = originalFetch
  })

  globalThis.fetch = (async () => new Response('not json', { status: 200 })) as typeof fetch

  await assert.rejects(
    apiRequest<DataEnvelope<unknown>, TestApiError>('/api/test', { method: 'GET' }, requestOptions()),
    (error) => {
      assert.equal(error instanceof TestApiError, true)
      assert.equal((error as TestApiError).code, 'invalid_response')
      assert.equal((error as Error).message, 'invalid response')
      return true
    },
  )
})

function requestOptions() {
  return {
    error: TestApiError,
    fallbackCode: 'request_failed',
    invalidResponseMessage: 'invalid response',
    message: (payload: ApiErrorEnvelope) => `mapped: ${payload.error?.message ?? 'failed'}`,
  }
}
