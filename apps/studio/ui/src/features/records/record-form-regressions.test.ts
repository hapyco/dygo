import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import vm from 'node:vm'
import ts from 'typescript'

// Exercise the SFC's actual setup functions without duplicating its conversion logic.
// Vue refs and API calls are supplied at their boundary; AST selection avoids line-based fixtures.
function setupFunctions(path: string, names: string[], context: Record<string, unknown>) {
  const sfc = readFileSync(new URL(path, import.meta.url), 'utf8')
  const script = sfc.split('<script setup lang="ts">')[1].split('</script>')[0]
  const source = ts.createSourceFile('component.ts', script, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS)
  const declarations = source.statements.filter(statement => (
    ts.isFunctionDeclaration(statement) && statement.name && names.includes(statement.name.text)
  ))
  assert.equal(declarations.length, names.length)
  const code = ts.transpileModule(declarations.map(node => node.getText(source)).join('\n'), {
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.None },
  }).outputText
  const scope = vm.createContext(context)
  vm.runInContext(code, scope)
  return { scope, source }
}

const formPath = '../../pages/RecordFormPage.vue'
const valueFunctions = ['integerSubmitValue', 'numberSubmitValue', 'jsonSubmitValue', 'stringSubmitValue', 'blankSubmitValue', 'initialFieldValue', 'editorForField', 'displayJSON', 'cloneCollectionRows', 'isRecordData', 'collectionCellSubmitValue', 'setCollectionError', 'isBlankValue']
function formValues(isNew = false) {
  return setupFunctions(formPath, valueFunctions, {
    isNew: { value: isNew }, recordFieldLabel: (field: { name: string }) => field.name,
  }).scope
}
function plain(value: unknown) { return JSON.parse(JSON.stringify(value)) }

for (const [kind, type, converter] of [
  ['integer', 'int', 'integerSubmitValue'], ['number', 'decimal', 'numberSubmitValue'],
  ['json', 'json', 'jsonSubmitValue'], ['date', 'date', 'stringSubmitValue'],
  ['datetime', 'datetime', 'stringSubmitValue'], ['time', 'time', 'stringSubmitValue'],
  ['string', 'select', 'stringSubmitValue'], ['integer', 'link', 'stringSubmitValue'],
]) {
  test(`clearing an optional ${type} persists null on Records and existing collection rows`, () => {
    const scope = formValues()
    const field = { name: 'value', type, 'value-kind': kind, required: false }
    const errors = {}
    assert.deepEqual(plain(scope[converter](field, '', errors)), { value: null })
    assert.deepEqual(plain(scope.collectionCellSubmitValue({ name: 'rows' }, field, '', 0, true, errors)), { value: null })
    assert.deepEqual(errors, {})
    assert.deepEqual(plain(formValues(true)[converter](field, '', {})), { skip: true })
    const requiredErrors = {}
    assert.deepEqual(plain(scope[converter]({ ...field, required: true }, '', requiredErrors)), { skip: true })
    assert.ok(Object.keys(requiredErrors).length)
  })
}

test('empty text remains empty text and an existing optional password is preserved', () => {
  const scope = formValues()
  assert.deepEqual(plain(scope.stringSubmitValue({ name: 'text', type: 'text', 'value-kind': 'string' }, '', {})), { value: '' })
  assert.deepEqual(plain(scope.collectionCellSubmitValue({ name: 'rows' }, { name: 'password', type: 'password', 'value-kind': 'password' }, '', 0, true, {})), { skip: true })
})

test('defaults initialize new forms without overwriting saved null values', () => {
  const scope = formValues()
  const field = { name: 'amount', type: 'int', 'value-kind': 'integer', default: 5 }
  assert.equal(scope.initialFieldValue(field, null), 5)
  assert.equal(scope.initialFieldValue(field, { amount: null }), '')
  assert.equal(scope.initialFieldValue(field, { amount: 0 }), 0)
})

test('untouched valid new forms can create defaults-only or empty Records', async () => {
  for (const payload of [{ amount: 5 }, {}]) {
    const calls: unknown[] = []
    const { scope, source } = setupFunctions(formPath, ['saveRecord'], {
      computed: (get: () => unknown) => ({ get value() { return get() } }),
      showForm: { value: true }, dirty: { value: false }, loading: { value: false }, createdRecord: { value: null },
      openCreatedRecord: async () => {},
      saving: { value: false }, isSystem: { value: false }, isNew: { value: true }, isSingle: { value: false },
      fieldErrors: { value: {} }, localError: { value: '' }, props: { entity: 'sample' },
      resetRecordActionErrors() {}, buildSubmitPayload: () => payload,
      createRecordMutation: { mutateAsync: async (input: unknown) => { calls.push(input); return { id: 1, name: 'SAMPLE-1' } } },
      resetToRecord() {}, toast: { success() {} }, router: { replace: async () => {} },
      RouteName: { RecordDetail: 'record' },
    })
    const canSave = source.statements.find(statement => ts.isVariableStatement(statement)
      && statement.declarationList.declarations.some(declaration => declaration.name.getText(source) === 'canSave'))
    assert.ok(canSave)
    vm.runInContext(canSave.getText(source), scope)
    assert.equal(vm.runInContext('canSave.value', scope), true)
    await scope.saveRecord()
    assert.deepEqual(plain(calls), [{ entity: 'sample', data: payload }])
  }
})

test('attachment upload uses saved normal and Single Record identities, never unsaved Records', async () => {
  for (const mode of ['record', 'single', 'new']) {
    const calls: unknown[][] = []
    const { scope } = setupFunctions('../../renderers/records/RecordFormRenderer.vue', ['attachmentUpload'], {
      props: { mode, record: { id: 42 }, appName: 'example', entityKey: 'settings' },
      uploadRecordFile: async (...args: unknown[]) => { calls.push(args); return { id: 7 } },
    })
    const upload = scope.attachmentUpload({ name: 'logo' })
    if (mode === 'new') { assert.equal(upload, undefined); continue }
    const file = { name: 'logo.png' }
    assert.equal(await upload(file), '7')
    assert.deepEqual(calls, [['example', 'settings', 42, 'logo', file]])
    scope.props.record = null
    assert.equal(scope.attachmentUpload({ name: 'logo' }), undefined)
  }
})

function createScope(replace: (target: unknown) => Promise<unknown>, mutate: () => Promise<unknown>) {
  const { scope, source } = setupFunctions(formPath, ['saveRecord', 'openCreatedRecord'], {
    computed: (get: () => unknown) => ({ get value() { return get() } }),
    showForm: { value: true }, dirty: { value: false }, loading: { value: false }, saving: { value: false },
    isSystem: { value: false }, isNew: { value: true }, isSingle: { value: false },
    createdRecord: { value: null }, openingCreatedRecord: { value: false },
    fieldErrors: { value: {} }, localError: { value: '' }, props: { entity: 'sample' },
    resetRecordActionErrors() {}, buildSubmitPayload: () => ({}), resetToRecord() {}, toast: { success() {} },
    createRecordMutation: { mutateAsync: mutate }, RouteName: { RecordDetail: 'record' },
    router: { currentRoute: { value: { path: '/sample/new' } }, resolve: () => ({ path: '/sample/SAMPLE-1' }), replace },
  })
  const canSave = source.statements.find(statement => ts.isVariableStatement(statement)
    && statement.declarationList.declarations.some(declaration => declaration.name.getText(source) === 'canSave'))
  assert.ok(canSave)
  vm.runInContext(canSave.getText(source), scope)
  return scope
}

test('committed creates survive aborted or rejected navigation without another POST', async () => {
  for (const navigation of ['aborted', 'rejected', 'redirected', 'success']) {
    let posts = 0
    let navigations = 0
    let recover = false
    const scope = createScope(async () => {
      navigations++
      if (recover || navigation === 'success') { scope.router.currentRoute.value.path = '/sample/SAMPLE-1'; return }
      if (navigation === 'aborted') return { type: 4 }
      if (navigation === 'rejected') throw new Error('navigation failed')
    }, async () => { posts++; return { id: 1, name: 'SAMPLE-1' } })
    await scope.saveRecord()
    assert.equal(posts, 1)
    assert.equal(vm.runInContext('canSave.value', scope), false)
    await scope.saveRecord()
    assert.equal(posts, 1)
    if (navigation !== 'success') {
      assert.match(scope.localError.value, /Record created.*Open created record/)
      recover = true
      await scope.openCreatedRecord()
      assert.equal(navigations, 2)
      assert.equal(posts, 1)
      assert.equal(scope.localError.value, '')
    }
  }
})

test('failed create mutations remain retryable', async () => {
  let posts = 0
  const scope = createScope(async () => {}, async () => {
    posts++
    if (posts === 1) throw new Error('temporary failure')
    return { id: 1, name: 'SAMPLE-1' }
  })
  await scope.saveRecord()
  assert.equal(scope.createdRecord.value, null)
  assert.equal(vm.runInContext('canSave.value', scope), true)
  await scope.saveRecord()
  assert.equal(posts, 2)
  assert.equal(vm.runInContext('canSave.value', scope), false)
})

test('new payload omits only untouched database defaults and preserves explicit inputs', () => {
  const makeField = (name: string, kind: string, type: string, extra: Record<string, unknown> = {}) => ({
    name, label: name, 'value-kind': kind, type, stored: true, required: false, studio: { editor: type }, ...extra,
  })
  const fields = [
    makeField('status', 'string', 'text', { default: 'draft', required: true }),
    makeField('amount', 'integer', 'int', { default: 5 }),
    makeField('enabled', 'boolean', 'boolean', { default: true }),
    makeField('count', 'integer', 'int', { default: 0 }),
    makeField('active', 'boolean', 'boolean', { default: false }),
    makeField('prefix', 'string', 'text', { default: 'INV' }),
    makeField('name', 'string', 'text', { required: true }),
    makeField('rows', 'json', 'collection', { stored: false }),
  ]
  const baseline = { status: 'draft', amount: 5, enabled: true, count: 0, active: false, prefix: 'INV', name: '', rows: [] }
  const { scope } = setupFunctions(formPath, [...valueFunctions, 'buildSubmitPayload', 'convertSubmitValue', 'draftValuesEqual'], {
    fields: { value: fields }, systemFields: { value: [] },
    entityMeta: { value: { naming: { strategy: 'format', format: '{prefix}-{count}' } } },
    isNew: { value: true }, baseline: { value: baseline },
    draft: { value: { ...baseline, amount: 0, enabled: false, name: 'explicit-name', rows: [{ quantity: 0 }] } },
    fieldErrors: { value: {} }, isHiddenRecordSubmitField: () => false,
    collectionSubmitValue: (_field: unknown, value: unknown) => ({ value }),
  })
  assert.deepEqual(plain(scope.buildSubmitPayload()), {
    amount: 0, enabled: false, prefix: 'INV', count: 0, name: 'explicit-name', rows: [{ quantity: 0 }],
  })
  assert.deepEqual(plain(scope.fieldErrors.value), {})
  scope.entityMeta.value.naming = { strategy: 'manual' }
  scope.draft.value.name = ''
  scope.buildSubmitPayload()
  assert.equal(scope.fieldErrors.value.name, 'Enter a value.')
})
