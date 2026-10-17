import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import vm from 'node:vm'
import ts from 'typescript'

// Execute the real mutation factories; replace only Vue Query, API, sound, and cache boundaries.
const source = readFileSync(new URL('../records/record-form.query.ts', import.meta.url), 'utf8')
const code = ts.transpileModule(source, {
  compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS },
}).outputText

const cases = [
  { hook: 'useCreateRecordMutation', api: 'createRecord', sound: 'save', args: ['sample', { title: 'Updated' }], cache: 'detail' },
  { hook: 'useUpdateRecordMutation', api: 'updateRecord', sound: 'save', args: ['sample', 42, { title: 'Updated' }], cache: 'detail' },
  { hook: 'useUpdateSingleRecordMutation', api: 'updateSingleRecord', sound: 'save', args: ['sample', { title: 'Updated' }], cache: 'single' },
  { hook: 'useAddRecordCommentMutation', api: 'addRecordComment', sound: 'save', args: ['sample', 42, 'Comment'], cache: 'activity' },
  { hook: 'useDeleteRecordMutation', api: 'deleteRecord', sound: 'delete', args: ['sample', 42], cache: 'delete' },
] as const

for (const scenario of cases) {
  test(`${scenario.hook} calls its API and binds success/error sounds with cache updates`, async () => {
    const sounds: string[] = []
    const apiCalls: unknown[][] = []
    const cacheCalls: unknown[][] = []
    const result = scenario.cache === 'delete' ? undefined
      : scenario.cache === 'activity' ? { created: true } : { name: 'SAMPLE-1', title: 'Updated' }
    const variables = { entity: 'sample', recordName: scenario.hook === 'useUpdateRecordMutation' ? 'SAMPLE-OLD' : 'SAMPLE-1', id: 42, recordID: 42, data: { title: 'Updated' }, message: 'Comment' }
    const exports: Record<string, () => {
      mutationFn: (input: typeof variables) => Promise<unknown>
      onSuccess: (data: unknown, input: typeof variables) => void
      onError: (error: Error) => void
    }> = {}
    const modules: Record<string, unknown> = {
      vue: {},
      '@tanstack/vue-query': { useMutation: (options: unknown) => options },
      '@/features/sounds': { studioSounds: Object.fromEntries(['save', 'delete', 'error'].map((name) => [name, () => sounds.push(name)])) },
      '@/app/query': { queryClient: Object.fromEntries(['setQueryData', 'invalidateQueries', 'removeQueries'].map((name) => [name, (...args: unknown[]) => cacheCalls.push([name, ...args])])) },
      './record-list.query': { recordListBaseQueryKey: (entity: string) => ['records', 'list', entity] },
      './records.api': { [scenario.api]: async (...args: unknown[]) => { apiCalls.push(args); return result } },
    }
    vm.runInNewContext(code, {
      exports,
      require: (name: string) => {
        assert.ok(name in modules, `unexpected import: ${name}`)
        return modules[name]
      },
    })
    const mutation = exports[scenario.hook]()
    assert.equal(await mutation.mutationFn(variables), result)
    assert.deepEqual(apiCalls, [scenario.args])
    assert.deepEqual(sounds, [], 'starting a request does not announce success')

    mutation.onError(new Error('request failed'))
    assert.deepEqual(sounds, ['error'])
    assert.deepEqual(cacheCalls, [], 'failure must not publish successful cache updates')

    sounds.length = 0
    mutation.onSuccess(result, variables)
    assert.deepEqual(sounds, [scenario.sound])
    const detailKey = ['records', 'detail', 'sample', 'SAMPLE-1']
    const invalidations = [
      ['invalidateQueries', { queryKey: ['records', 'secret-status', 'sample'] }],
      ['invalidateQueries', { queryKey: ['records', 'list', 'sample'] }],
    ]
    const expected = scenario.cache === 'activity'
      ? [['invalidateQueries', { queryKey: ['records', 'activity', 'sample', '42'] }]]
      : scenario.cache === 'delete'
        ? [['removeQueries', { queryKey: detailKey, exact: true }], ...invalidations]
        : [
          ...(scenario.cache === 'single' ? [['setQueryData', ['records', 'single', 'sample'], result]] : []),
          ...(scenario.hook === 'useUpdateRecordMutation' ? [['setQueryData', ['records', 'detail', 'sample', 'SAMPLE-OLD'], result]] : []),
          ['setQueryData', detailKey, result],
          ...invalidations,
        ]
    assert.deepEqual(cacheCalls.map((call) => JSON.stringify(call)).sort(), expected.map((call) => JSON.stringify(call)).sort())
  })
}
