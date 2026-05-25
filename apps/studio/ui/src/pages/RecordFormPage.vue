<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Plus, RotateCcw, Save } from '@lucide/vue'

import { ErrorState, Spinner } from '@/design'
import type { MetadataEntityMeta, MetadataField } from '@/features/metadata/metadata.api'
import type { RecordData } from '@/features/records/records.api'
import { isHiddenRecordSubmitField } from '@/features/records/system-fields'
import { RecordFormRenderer } from '@/renderers/records'
import { RouteName } from '@/router/routes'
import PageHeader from '@/shell/PageHeader.vue'
import type { PageHeaderAction } from '@/shell/types'
import { useMetadataStore } from '@/stores/metadata.store'
import { singleRecordKey, useRecordsStore } from '@/stores/records.store'

const props = defineProps<{
  entity: string
  recordName?: string
  mode: 'new' | 'record' | 'single'
}>()

type ConvertedValue = {
  skip?: boolean
  value?: unknown
}

const router = useRouter()
const metadataStore = useMetadataStore()
const recordsStore = useRecordsStore()

const draft = ref<RecordData>({})
const baseline = ref<RecordData>({})
const fieldErrors = ref<Record<string, string>>({})
const localError = ref('')

const recordStateKey = computed(() => {
  if (props.mode === 'new') {
    return 'new'
  }
  if (props.mode === 'single') {
    return singleRecordKey
  }
  return props.recordName ?? ''
})
const recordState = computed(() => recordsStore.recordState(props.entity, recordStateKey.value))
const entityMeta = computed(() => metadataStore.entityMeta(props.entity))
const entityMetaStatus = computed(() => metadataStore.entityMetaStatus(props.entity))
const entityMetaError = computed(() => metadataStore.entityMetaError(props.entity))
const systemFields = computed(() => entityMeta.value?.['system-fields'] ?? [])
const fields = computed(() => {
  const meta = entityMeta.value
  if (!meta) {
    return []
  }

  if (meta.naming?.strategy !== 'manual') {
    return meta.fields
  }

  const nameField = manualNameField(meta.naming?.label)
  return nameField ? [nameField, ...meta.fields] : meta.fields
})
const entityLabel = computed(() => entityMeta.value?.label || humanizeEntity(props.entity))
const isSystem = computed(() => entityMeta.value?.['is-system'] === true)
const isNew = computed(() => props.mode === 'new')
const isSingle = computed(() => props.mode === 'single')
const loading = computed(() => (
  entityMetaStatus.value === 'idle'
  || entityMetaStatus.value === 'loading'
  || (!isNew.value && (recordState.value.status === 'idle' || recordState.value.status === 'loading'))
))
const saving = computed(() => recordState.value.saving)
const blockingError = computed(() => entityMetaError.value?.message ?? recordState.value.error?.message ?? '')
const saveError = computed(() => localError.value || recordState.value.saveError?.message || '')
const showForm = computed(() => Boolean(entityMeta.value) && (isNew.value || Boolean(recordState.value.record)))
const dirty = computed(() => fields.value.some((field) => !draftValuesEqual(draft.value[field.name], baseline.value[field.name])))
const canSave = computed(() => showForm.value && dirty.value && !loading.value && !saving.value && !isSystem.value)
const actions = computed<PageHeaderAction[]>(() => {
  if (isSystem.value) {
    return []
  }

  return [
    {
      label: 'Reset',
      icon: RotateCcw,
      variant: 'secondary',
      disabled: !dirty.value || loading.value || saving.value,
      onSelect: resetDraft,
    },
    {
      label: isNew.value ? 'Create record' : 'Save',
      icon: isNew.value ? Plus : Save,
      variant: 'primary',
      disabled: !canSave.value,
      loading: saving.value,
      onSelect: saveRecord,
    },
  ]
})

watch(
  () => [props.entity, props.mode, props.recordName] as const,
  async ([entity, mode, recordName]) => {
    fieldErrors.value = {}
    localError.value = ''
    const meta = await metadataStore.loadEntityMeta(entity)

    if (meta?.['is-single'] && mode !== 'single') {
      await router.replace({ name: RouteName.EntityRecords, params: { entity } })
      return
    }

    if (meta?.['is-system'] && mode === 'new') {
      await router.replace({ name: RouteName.EntityRecords, params: { entity } })
      return
    }

    if (mode === 'single') {
      await recordsStore.loadSingleRecord(entity)
    } else if (mode === 'record' && recordName) {
      await recordsStore.loadRecordByName(entity, recordName)
    } else if (mode === 'new') {
      recordsStore.resetRecordForm(entity, 'new')
    }
  },
  { immediate: true },
)

watch(
  () => [entityMeta.value, recordState.value.record, props.mode] as const,
  ([meta, record, mode]) => {
    if (!meta) {
      return
    }

    const nextDraft = mode === 'new'
      ? draftFromDefaults(meta.fields)
      : draftFromRecord(meta.fields, record)

    draft.value = nextDraft
    baseline.value = { ...nextDraft }
    fieldErrors.value = {}
    localError.value = ''
  },
  { immediate: true },
)

function resetDraft() {
  draft.value = { ...baseline.value }
  fieldErrors.value = {}
  localError.value = ''
}

function updateDraft(value: RecordData) {
  draft.value = value
}

async function saveRecord() {
  if (!canSave.value) {
    return
  }

  fieldErrors.value = {}
  localError.value = ''

  const payload = buildSubmitPayload()
  if (Object.keys(fieldErrors.value).length > 0) {
    return
  }

  if (Object.keys(payload).length === 0) {
    return
  }

  try {
    const record = isNew.value
      ? await recordsStore.createRecord(props.entity, payload)
      : isSingle.value
        ? await recordsStore.updateSingleRecord(props.entity, payload)
        : await recordsStore.updateRecord(props.entity, props.recordName ?? '', currentRecordID(), payload)

    resetToRecord(record)
    const nextName = typeof record.name === 'string' ? record.name : ''
    if (!isSingle.value && nextName && (isNew.value || nextName !== props.recordName)) {
      await router.replace({ name: RouteName.RecordDetail, params: { entity: props.entity, recordName: nextName } })
    }
  } catch {
    // The store owns the API error shape for display.
  }
}

function currentRecordID(): string | number {
  const id = recordState.value.record?.id
  if (typeof id === 'string' || typeof id === 'number') {
    return id
  }

  localError.value = 'This record is missing its internal ID.'
  throw new Error('record id is missing')
}

function buildSubmitPayload(): RecordData {
  const payload: RecordData = {}
  const errors: Record<string, string> = {}

  fields.value.forEach((field) => {
    if (isHiddenRecordSubmitField(field.name, systemFields.value)) {
      return
    }

    if (!isNew.value && field.name === 'name') {
      return
    }

    if (!isNew.value && draftValuesEqual(draft.value[field.name], baseline.value[field.name])) {
      return
    }

    const converted = convertSubmitValue(field, draft.value[field.name], errors)
    if (!converted.skip) {
      payload[field.name] = converted.value
    }
  })

  fieldErrors.value = errors
  return payload
}

function convertSubmitValue(field: MetadataField, value: unknown, errors: Record<string, string>): ConvertedValue {
  if (editorForField(field) === 'collection') {
    return collectionSubmitValue(field, value, errors)
  }

  if (field.type === 'link') {
    return stringSubmitValue(field, value)
  }

  if (field.studio?.editor === 'select' && (value === undefined || value === null || value === '') && field.required) {
    errors[field.name] = 'Select a value.'
    return { skip: true }
  }

  switch (field['value-kind']) {
    case 'password':
      if (typeof value !== 'string' || value.length === 0) {
        return { skip: true }
      }
      return { value }
    case 'integer':
      return integerSubmitValue(field, value, errors)
    case 'number':
      return numberSubmitValue(field, value, errors)
    case 'boolean':
      return { value: value === true }
    case 'json':
      return jsonSubmitValue(field, value, errors)
    case 'date':
    case 'datetime':
    case 'time':
    case 'string':
      return stringSubmitValue(field, value)
    default:
      return { skip: true }
  }
}

function collectionSubmitValue(field: MetadataField, value: unknown, errors: Record<string, string>): ConvertedValue {
  if (!Array.isArray(value)) {
    errors[field.name] = 'Use rows for this field.'
    return { skip: true }
  }

  if (field.required && value.length === 0) {
    errors[field.name] = 'Add at least one row.'
    return { skip: true }
  }

  if (isNew.value && value.length === 0 && !field.required) {
    return { skip: true }
  }

  const childMeta = entityMeta.value?.collections?.[field.name]
  if (!childMeta) {
    errors[field.name] = 'Collection metadata is missing.'
    return { skip: true }
  }

  const rows: RecordData[] = []
  value.forEach((row, index) => {
    if (!isRecordData(row)) {
      setCollectionError(errors, field, index, 'row is invalid.')
      return
    }

    const converted = collectionRowSubmitValue(field, childMeta, row, index, errors)
    if (converted) {
      rows.push(converted)
    }
  })

  if (errors[field.name]) {
    return { skip: true }
  }

  return { value: rows }
}

function collectionRowSubmitValue(parentField: MetadataField, childMeta: MetadataEntityMeta, row: RecordData, rowIndex: number, errors: Record<string, string>): RecordData | null {
  const output: RecordData = {}
  const id = row.id
  const existing = typeof id === 'number' || (typeof id === 'string' && id.length > 0)
  if (existing) {
    output.id = id
  }

  childMeta.fields.forEach((field) => {
    if (isHiddenCollectionSubmitField(field)) {
      return
    }

    const converted = collectionCellSubmitValue(parentField, field, row[field.name], rowIndex, existing, errors)
    if (!converted.skip) {
      output[field.name] = converted.value
    }
  })

  if (errors[parentField.name]) {
    return null
  }

  return output
}

function collectionCellSubmitValue(parentField: MetadataField, field: MetadataField, value: unknown, rowIndex: number, existing: boolean, errors: Record<string, string>): ConvertedValue {
  if (isBlankValue(value)) {
    if (field.required) {
      setCollectionError(errors, parentField, rowIndex, `${labelForField(field)} is required.`)
      return { skip: true }
    }
    if (existing && ['string', 'date', 'datetime', 'time'].includes(field['value-kind'])) {
      return { value: '' }
    }
    return { skip: true }
  }

  if (field.type === 'link') {
    return { value: String(value) }
  }

  switch (field['value-kind']) {
    case 'password':
      return { value: String(value) }
    case 'integer': {
      const number = Number(value)
      if (!Number.isInteger(number)) {
        setCollectionError(errors, parentField, rowIndex, `${labelForField(field)} must be an integer.`)
        return { skip: true }
      }
      return { value: number }
    }
    case 'number': {
      const number = Number(value)
      if (!Number.isFinite(number)) {
        setCollectionError(errors, parentField, rowIndex, `${labelForField(field)} must be a number.`)
        return { skip: true }
      }
      return { value: number }
    }
    case 'boolean':
      return { value: value === true }
    case 'json':
      if (typeof value !== 'string') {
        return { value }
      }
      try {
        return { value: JSON.parse(value) }
      } catch {
        setCollectionError(errors, parentField, rowIndex, `${labelForField(field)} must be valid JSON.`)
        return { skip: true }
      }
    case 'date':
    case 'datetime':
    case 'time':
    case 'string':
      return { value: String(value) }
    default:
      return { skip: true }
  }
}

function setCollectionError(errors: Record<string, string>, field: MetadataField, rowIndex: number, message: string) {
  if (!errors[field.name]) {
    errors[field.name] = `Row ${rowIndex + 1}: ${message}`
  }
}

function stringSubmitValue(field: MetadataField, value: unknown): ConvertedValue {
  const text = value === null || value === undefined ? '' : String(value)
  if (isNew.value && text === '' && !field.required) {
    return { skip: true }
  }

  return { value: text }
}

function integerSubmitValue(field: MetadataField, value: unknown, errors: Record<string, string>): ConvertedValue {
  if (value === null || value === undefined || value === '') {
    if (field.required) {
      errors[field.name] = 'Enter an integer.'
    }
    return { skip: true }
  }

  const number = Number(value)
  if (!Number.isInteger(number)) {
    errors[field.name] = 'Enter an integer.'
    return { skip: true }
  }

  return { value: number }
}

function numberSubmitValue(field: MetadataField, value: unknown, errors: Record<string, string>): ConvertedValue {
  if (value === null || value === undefined || value === '') {
    if (field.required) {
      errors[field.name] = 'Enter a number.'
    }
    return { skip: true }
  }

  const number = Number(value)
  if (!Number.isFinite(number)) {
    errors[field.name] = 'Enter a number.'
    return { skip: true }
  }

  return { value: number }
}

function jsonSubmitValue(field: MetadataField, value: unknown, errors: Record<string, string>): ConvertedValue {
  if (value === null || value === undefined || value === '') {
    if (field.required) {
      errors[field.name] = 'Enter valid JSON.'
    }
    return { skip: true }
  }

  if (typeof value !== 'string') {
    return { value }
  }

  try {
    return { value: JSON.parse(value) }
  } catch {
    errors[field.name] = 'Enter valid JSON.'
    return { skip: true }
  }
}

function resetToRecord(record: RecordData) {
  const nextDraft = draftFromRecord(fields.value, record)
  draft.value = nextDraft
  baseline.value = { ...nextDraft }
}

function draftFromDefaults(metadataFields: MetadataField[]): RecordData {
  return metadataFields.reduce<RecordData>((next, field) => {
    next[field.name] = initialFieldValue(field, null)
    return next
  }, {})
}

function draftFromRecord(metadataFields: MetadataField[], record: RecordData | null): RecordData {
  return metadataFields.reduce<RecordData>((next, field) => {
    next[field.name] = initialFieldValue(field, record)
    return next
  }, {})
}

function initialFieldValue(field: MetadataField, record: RecordData | null): unknown {
  if (editorForField(field) === 'collection') {
    const recordValue = record?.[field.name]
    return Array.isArray(recordValue) ? cloneCollectionRows(recordValue) : []
  }

  if (field['write-only']) {
    return ''
  }

  const recordValue = record?.[field.name]
  if (recordValue !== undefined && recordValue !== null) {
    return field['value-kind'] === 'json' ? displayJSON(recordValue) : recordValue
  }

  if (field.default !== undefined) {
    return field['value-kind'] === 'json' ? displayJSON(field.default) : field.default
  }

  if (field['value-kind'] === 'boolean') {
    return false
  }

  return ''
}

function editorForField(field: MetadataField): string {
  return field.studio?.editor || field.type
}

function labelForField(field: MetadataField): string {
  return field.label || field.name
}

function isBlankValue(value: unknown): boolean {
  return value === null || value === undefined || value === ''
}

function isRecordData(value: unknown): value is RecordData {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function cloneCollectionRows(value: unknown[]): RecordData[] {
  return value.filter(isRecordData).map((row) => ({ ...row }))
}

function isHiddenCollectionSubmitField(field: MetadataField): boolean {
  // TODO(collections): add nested collection editing after the v1 child-table contract settles.
  if (field.type === 'collection') {
    return true
  }

  return [
    'id',
    'name',
    'created-at',
    'updated-at',
    'parent-entity-id',
    'parent_entity_id',
    'parent-record-id',
    'parent_record_id',
    'parent-field-id',
    'parent_field_id',
    'ordinal',
  ].includes(field.name)
}

function displayJSON(value: unknown): string {
  if (typeof value === 'string') {
    return value
  }

  return JSON.stringify(value, null, 2)
}

function manualNameField(label?: string): MetadataField | null {
  const field = systemFields.value.find((candidate) => candidate.name === 'name')
  if (!field) {
    return null
  }

  return {
    ...field,
    label: label || field.label,
    required: true,
  }
}

function draftValuesEqual(left: unknown, right: unknown): boolean {
  return JSON.stringify(left ?? '') === JSON.stringify(right ?? '')
}

function humanizeEntity(value: string): string {
  return value
    .replace(/[-_]+/g, ' ')
    .replace(/\b\w/g, (letter) => letter.toUpperCase())
}
</script>

<template>
  <section class="studio-page record-form-page" :aria-label="entityLabel">
    <PageHeader
      :show-title="false"
      :actions="actions"
    />

    <div class="record-form-page__body">
      <div v-if="loading" class="record-form-page__state">
        <Spinner size="sm" label="Loading record" />
        <p>Loading record</p>
      </div>

      <ErrorState
        v-else-if="blockingError && !showForm"
        title="Record unavailable"
        :message="blockingError"
      />

      <template v-else-if="entityMeta">
        <ErrorState
          v-if="saveError"
          title="Record not saved"
          :message="saveError"
        />

        <RecordFormRenderer
          :entity="props.entity"
          :entity-label="entityLabel"
          :fields="fields"
          :system-fields="systemFields"
          :collections="entityMeta.collections"
          :record="recordState.record"
          :mode="props.mode"
          :model-value="draft"
          :field-errors="fieldErrors"
          :disabled="saving || isSystem"
          @update:model-value="updateDraft"
        />
      </template>
    </div>
  </section>
</template>

<style scoped>
.record-form-page {
  gap: 0;
  grid-template-rows: auto minmax(0, 1fr);
  height: 100%;
  min-height: 0;
}

.record-form-page__body {
  min-height: 0;
  overflow: auto;
}

.record-form-page__state {
  display: grid;
  justify-items: start;
  gap: 10px;
  padding: 196px 16px 44px;
}

.record-form-page__state p {
  margin: 0;
  color: var(--studio-text-muted);
  font-size: 13px;
  font-weight: 500;
}
</style>
