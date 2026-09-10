<script setup lang="ts">
import { computed, onScopeDispose, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { usePageCommands, runStudioCommand } from '@/features/commands/context'
import { bindings } from '@/features/commands/shortcuts'
import { useDraftGuard } from '@/features/records/use-draft-guard'
import { Ban, Play, Plus, RotateCcw, Save, Trash2 } from '@lucide/vue'

import { queryClient } from '@/app/query'
import { ErrorState, Spinner } from '@/design'
import { useDialog } from '@/features/dialogs/use-dialog'
import { useToast } from '@/features/toasts/use-toast'
import type { EntityActionDefinition, MetadataEntityMeta, MetadataField } from '@/features/metadata/metadata.api'
import { useMetadataEntityMetaQuery } from '@/features/metadata/metadata.query'
import {
  entityActionConfirm,
  entityActionDisabledReason,
  recordEntityActions,
} from '@/features/records/entity-actions'
import { useExecuteRecordActionMutation } from '@/features/records/record-actions.query'
import {
  recordActivityQueryKey,
  recordByNameQueryKey,
  useSecretStatusQuery,
  useCreateRecordMutation,
  useAddRecordCommentMutation,
  useDeleteRecordMutation,
  useRecordByNameQuery,
  useSingleRecordQuery,
  useUpdateRecordMutation,
  useUpdateSingleRecordMutation,
  useRecordActivityQuery,
} from '@/features/records/record-form.query'
import { recordListBaseQueryKey } from '@/features/records/record-list.query'
import type { RecordData } from '@/features/records/records.api'
import { secretSubmitValue } from '@/features/records/secret-input'
import { isHiddenCollectionField, isHiddenRecordSubmitField, recordFieldLabel } from '@/features/records/system-fields'
import { RecordFormRenderer, RecordTimeline } from '@/renderers/records'
import { RouteName } from '@/router/routes'
import PageHeader from '@/shell/PageHeader.vue'
import type { PageHeaderAction } from '@/shell/types'
import { humanizeEntity } from '@/stores/metadata.identity'
import { statusForError, storeError, type LoadStatus } from '@/stores/status'
import type { PinnedItem } from '@/features/pinned/pinned'
import { useSheetActive } from '@/features/tabs/sheet-active'
import { useNavigationStore } from '@/stores/navigation.store'

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
const route = useRoute()
const navigation = useNavigationStore()
const sheetActive = useSheetActive()
const sheetPath = route.path
const dialog = useDialog()
const toast = useToast()
const entityMetaQuery = useMetadataEntityMetaQuery(() => props.entity)

const draft = ref<RecordData>({})
const baseline = ref<RecordData>({})
const fieldErrors = ref<Record<string, string>>({})
const localError = ref('')
const entityMeta = computed(() => entityMetaQuery.data.value ?? null)
const entityMetaError = computed(() => (
  entityMetaQuery.error.value
    ? storeError(entityMetaQuery.error.value, 'Studio could not load entity metadata.')
    : null
))
const entityMetaStatus = computed<LoadStatus>(() => {
  if (entityMetaQuery.isPending.value) {
    return 'loading'
  }

  if (entityMetaError.value) {
    return statusForError(entityMetaError.value)
  }

  return entityMeta.value ? 'ready' : 'idle'
})
const isNew = computed(() => props.mode === 'new')
const isSingle = computed(() => props.mode === 'single')
const isRecord = computed(() => props.mode === 'record')
const recordByNameQuery = useRecordByNameQuery(
  () => props.entity,
  () => props.recordName ?? '',
  {
    enabled: computed(() => (
      isRecord.value
      && entityMeta.value?.slug === props.entity
      && entityMeta.value?.['is-single'] !== true
    )),
  },
)
const singleRecordQuery = useSingleRecordQuery(
  () => props.entity,
  {
    enabled: computed(() => (
      isSingle.value
      && entityMeta.value?.slug === props.entity
    )),
  },
)
const createRecordMutation = useCreateRecordMutation()
const updateRecordMutation = useUpdateRecordMutation()
const updateSingleRecordMutation = useUpdateSingleRecordMutation()
const deleteRecordMutation = useDeleteRecordMutation()
const addCommentMutation = useAddRecordCommentMutation()
const entityActionMutation = useExecuteRecordActionMutation()
const record = computed(() => {
  if (isSingle.value) {
    return singleRecordQuery.data.value ?? null
  }

  if (isRecord.value) {
    return recordByNameQuery.data.value ?? null
  }

  return null
})
const timelineRecordID = computed(() => {
  const id = record.value?.id
  return !isNew.value && (typeof id === 'string' || typeof id === 'number') ? id : 0
})
const timelineQuery = useRecordActivityQuery(
  () => props.entity,
  timelineRecordID,
  { enabled: computed(() => Number(timelineRecordID.value) > 0) },
)
const secretStatusQuery = useSecretStatusQuery(() => props.entity, () => Number(record.value?.id) || 0, computed(() => Boolean(
  entityMeta.value?.fields.some(field => field.type === 'secret') || Object.values(entityMeta.value?.collections ?? {}).some(child => child.fields.some(field => field.type === 'secret'))
)))
const secretStatus = computed(() => secretStatusQuery.data.value)
const commentDraft = ref('')
const timelineError = computed(() => timelineQuery.error.value?.message || addCommentMutation.error.value?.message || '')
const timelineEntries = computed(() => timelineQuery.data.value?.data ?? [])
const recordError = computed(() => {
  if (isSingle.value && singleRecordQuery.error.value) {
    return storeError(singleRecordQuery.error.value, 'Studio could not load these settings.')
  }

  if (isRecord.value && recordByNameQuery.error.value) {
    return storeError(recordByNameQuery.error.value, 'Studio could not load this record.')
  }

  return null
})
const recordStatus = computed<LoadStatus>(() => {
  if (isNew.value) {
    return 'ready'
  }

  const activeQuery = isSingle.value ? singleRecordQuery : recordByNameQuery
  if (activeQuery.isPending.value) {
    return 'loading'
  }

  if (recordError.value) {
    return statusForError(recordError.value)
  }

  return record.value ? 'ready' : 'idle'
})
const recordActionError = computed(() => {
  if (createRecordMutation.error.value) {
    return storeError(createRecordMutation.error.value, 'Studio could not create this record.')
  }

  if (updateSingleRecordMutation.error.value) {
    return storeError(updateSingleRecordMutation.error.value, 'Studio could not save these settings.')
  }

  if (updateRecordMutation.error.value) {
    return storeError(updateRecordMutation.error.value, 'Studio could not save this record.')
  }

  if (deleteRecordMutation.error.value) {
    return storeError(deleteRecordMutation.error.value, 'Studio could not delete this record.')
  }

  if (entityActionMutation.error.value) {
    return storeError(entityActionMutation.error.value, 'Studio could not run this action.')
  }

  return null
})
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
const pinTarget = computed<PinnedItem | null>(() => {
  if (!entityMeta.value || isNew.value) return null
  if (isSingle.value) return {
    type: 'entity', app: entityMeta.value.app.name, entity: entityMeta.value.key,
    label: entityLabel.value, path: `/${props.entity}`,
  }
  if (!props.recordName || !record.value) return null
  return {
    type: 'record', app: entityMeta.value.app.name, entity: entityMeta.value.key, record: props.recordName,
    label: `${entityLabel.value} / ${props.recordName}`, path: `/${props.entity}/${encodeURIComponent(props.recordName)}`,
  }
})
const isSystem = computed(() => entityMeta.value?.['is-system'] === true)
const loading = computed(() => (
  entityMetaStatus.value === 'idle'
  || entityMetaStatus.value === 'loading'
  || (!isNew.value && (recordStatus.value === 'idle' || recordStatus.value === 'loading'))
))
const saving = computed(() => (
  createRecordMutation.isPending.value
  || updateRecordMutation.isPending.value
  || updateSingleRecordMutation.isPending.value
  || deleteRecordMutation.isPending.value
))
const blockingError = computed(() => entityMetaError.value?.message ?? recordError.value?.message ?? '')
const saveError = computed(() => localError.value || recordActionError.value?.message || '')
const showForm = computed(() => Boolean(entityMeta.value) && (isNew.value || Boolean(record.value)))
const dirty = computed(() => fields.value.some((field) => !draftValuesEqual(draft.value[field.name], baseline.value[field.name])))
const canSave = computed(() => showForm.value && dirty.value && !loading.value && !saving.value && !isSystem.value)
const confirmDiscard = useDraftGuard(() => dirty.value, () => saving.value)
watch([dirty, sheetActive], () => {
  if (!sheetActive.value) return
  navigation.setTabDirty(sheetPath, dirty.value)
}, { immediate: true })
onScopeDispose(() => navigation.setTabDirty(sheetPath, false))
const saveDisabledReason = computed(() => isSystem.value ? 'Read-only Record' : loading.value ? 'Loading Record' : saving.value ? 'Saving Record' : !showForm.value ? 'Record unavailable' : !dirty.value ? 'No changes' : undefined)
usePageCommands(computed(() => [
  { id: 'record:save', label: isNew.value ? 'Create Record' : 'Save Record', disabledReason: entityActionMutation.isPending.value ? 'Action is running' : saveDisabledReason.value, run: saveRecord },
  { id: 'record:reset', label: 'Reset changes', disabledReason: !dirty.value ? 'No changes' : loading.value || saving.value || entityActionMutation.isPending.value ? 'Record is busy' : isSystem.value ? 'Read-only Record' : undefined, run: resetDraft },
  ...(!isSingle.value ? [{ id: 'record:list', label: 'Go to Entity list', run: async () => { await router.push({ name: RouteName.EntityRecords, params: { entity: props.entity } }) } }] : []),
]))
const entityActions = computed(() => recordEntityActions(entityMeta.value?.actions))
const actions = computed<PageHeaderAction[]>(() => {
  const next: PageHeaderAction[] = []
  if (!isSystem.value) {
    next.push(
      {
        label: 'Reset',
        icon: RotateCcw,
        variant: 'secondary',
        disabled: !dirty.value || loading.value || saving.value || entityActionMutation.isPending.value,
        onSelect: () => { void runStudioCommand('record:reset') },
      },
      {
        label: isNew.value ? 'Create record' : 'Save',
        icon: isNew.value ? Plus : Save,
        variant: 'primary',
        disabled: !canSave.value || entityActionMutation.isPending.value,
        loading: saving.value,
        shortcut: bindings['record:save']?.shortcut,
        onSelect: () => { void runStudioCommand('record:save') },
      },
    )
    if (isRecord.value) {
      next.push({
        label: 'Delete',
        icon: Trash2,
        variant: 'secondary',
        disabled: loading.value || saving.value || entityActionMutation.isPending.value || !record.value,
        loading: saving.value,
        onSelect: deleteRecord,
      })
    }
  }

  if (isRecord.value) {
    entityActions.value.forEach((action) => {
      if (!action) {
        return
      }
      const disabledReason = entityActionDisabledReason(entityMeta.value?.key ?? '', action.name, record.value)
      next.push({
        label: action.label,
        icon: entityActionIcon(action.name),
        variant: action.danger ? 'danger' : 'secondary',
        disabled: loading.value || saving.value || entityActionMutation.isPending.value || !record.value || Boolean(disabledReason),
        loading: entityActionMutation.isPending.value,
        onSelect: () => { void runEntityAction(action) },
      })
    })
  }

  return next
})

watch(
  () => [props.entity, props.mode, props.recordName] as const,
  () => {
    fieldErrors.value = {}
    localError.value = ''
    resetRecordActionErrors()
  },
  { immediate: true },
)

watch(
  () => [props.entity, props.mode, props.recordName, entityMeta.value] as const,
  async ([entity, mode, _recordName, meta]) => {
    if (!meta || meta.slug !== entity) {
      return
    }

    if (meta?.['is-single'] && mode !== 'single') {
      navigation.replaceTab(route.path, { path: `/${entity}`, fullPath: `/${entity}`, label: entityLabel.value })
      await router.replace({ name: RouteName.EntityRecords, params: { entity } })
      return
    }

    if (meta?.['is-system'] && mode === 'new') {
      navigation.replaceTab(route.path, { path: `/${entity}`, fullPath: `/${entity}`, label: entityLabel.value })
      await router.replace({ name: RouteName.EntityRecords, params: { entity } })
      return
    }
  },
  { immediate: true },
)

let draftIdentity = ''
watch(
  () => [entityMeta.value, record.value, props.mode, props.entity, props.recordName] as const,
  ([meta, nextRecord, mode, entity]) => {
    if (!meta || meta.slug !== entity) {
      return
    }

    if (mode !== 'new' && !nextRecord) {
      return
    }

    const identity = `${entity}:${mode}:${props.recordName ?? ''}`
    if (identity === draftIdentity && dirty.value) return
    draftIdentity = identity

    const nextDraft = mode === 'new'
      ? draftFromRecord(fields.value, null)
      : draftFromRecord(fields.value, nextRecord)

    draft.value = nextDraft
    baseline.value = { ...nextDraft }
    fieldErrors.value = {}
    localError.value = ''
  },
  { immediate: true },
)

async function resetDraft() {
  if (!dirty.value || loading.value || saving.value || isSystem.value || !await confirmDiscard()) return
  draft.value = { ...baseline.value }
  fieldErrors.value = {}
  localError.value = ''
}

function updateDraft(value: RecordData) {
  draft.value = value
}

function resetRecordActionErrors() {
  createRecordMutation.reset()
  updateRecordMutation.reset()
  updateSingleRecordMutation.reset()
  deleteRecordMutation.reset()
  addCommentMutation.reset()
  entityActionMutation.reset()
}

function entityActionIcon(name: string) {
  if (name === 'cancel') {
    return Ban
  }
  if (name === 'retry') {
    return RotateCcw
  }
  return Play
}

async function runEntityAction(action: EntityActionDefinition) {
  if (!isRecord.value || loading.value || saving.value || entityActionMutation.isPending.value || !record.value) {
    return
  }
  if (entityActionDisabledReason(entityMeta.value?.key ?? '', action.name, record.value)) {
    return
  }

  const confirm = entityActionConfirm(action)
  if (confirm) {
    const decision = await dialog.confirm({
      title: confirm.title,
      content: confirm.content,
      type: confirm.type,
      actions: [
        { key: 'cancel', label: 'Back', variant: 'secondary' },
        { key: 'confirm', label: action.label, variant: confirm.type === 'danger' ? 'danger' : 'primary' },
      ],
    })
    if (decision !== 'confirm') {
      return
    }
  }

  localError.value = ''
  resetRecordActionErrors()

  const recordID = Number(currentRecordID())
  try {
    await entityActionMutation.mutateAsync({
      entity: props.entity,
      action: action.name,
      records: [recordID],
    })
    await queryClient.invalidateQueries({ queryKey: recordByNameQueryKey(props.entity, props.recordName ?? '') })
    await queryClient.invalidateQueries({ queryKey: recordActivityQueryKey(props.entity, recordID) })
    await queryClient.invalidateQueries({ queryKey: recordListBaseQueryKey(props.entity) })
    toast.success(`${action.label} complete`)
  } catch {
    // TanStack owns the mutation error for display.
  }
}

async function addComment() {
  const recordID = timelineRecordID.value
  const message = commentDraft.value.trim()
  if (!recordID || !message || addCommentMutation.isPending.value) return

  try {
    await addCommentMutation.mutateAsync({ entity: props.entity, recordID, message })
    commentDraft.value = ''
    toast.success('Comment added')
  } catch {
    // TanStack owns the mutation error for display.
  }
}

async function saveRecord() {
  if (!canSave.value) {
    return
  }

  fieldErrors.value = {}
  localError.value = ''
  resetRecordActionErrors()

  const payload = buildSubmitPayload()
  if (Object.keys(fieldErrors.value).length > 0) {
    return
  }

  if (Object.keys(payload).length === 0) {
    return
  }

  try {
    const record = isNew.value
      ? await createRecordMutation.mutateAsync({ entity: props.entity, data: payload })
      : isSingle.value
        ? await updateSingleRecordMutation.mutateAsync({ entity: props.entity, data: payload })
        : await updateRecordMutation.mutateAsync({
            entity: props.entity,
            recordName: props.recordName ?? '',
            id: currentRecordID(),
            data: payload,
          })

    resetToRecord(record)
    toast.success(isSingle.value ? 'Settings saved' : isNew.value ? 'Record created' : 'Record saved')
    const nextName = typeof record.name === 'string' ? record.name : ''
    if (!isSingle.value && nextName && (isNew.value || nextName !== props.recordName)) {
      await router.replace({ name: RouteName.RecordDetail, params: { entity: props.entity, recordName: nextName } })
    }
  } catch {
    // TanStack owns the mutation error for display.
  }
}

async function deleteRecord() {
  if (!isRecord.value || loading.value || saving.value || !record.value) {
    return
  }

  const recordLabel = props.recordName || entityLabel.value
  const action = await dialog.confirm({
    title: `Delete ${recordLabel}?`,
    content: 'This cannot be undone.',
    type: 'danger',
    actions: [
      { key: 'cancel', label: 'Cancel', variant: 'secondary' },
      { key: 'confirm', label: 'Delete', variant: 'danger' },
    ],
  })
  if (action !== 'confirm') {
    return
  }

  localError.value = ''
  resetRecordActionErrors()

  try {
    await deleteRecordMutation.mutateAsync({
      entity: props.entity,
      recordName: props.recordName ?? '',
      id: currentRecordID(),
    })
    toast.success('Record deleted')
    navigation.replaceTab(sheetPath, { path: `/${props.entity}`, fullPath: `/${props.entity}`, label: entityLabel.value })
    await router.replace({ name: RouteName.EntityRecords, params: { entity: props.entity } })
  } catch {
    // TanStack owns the mutation error for display.
  }
}

function currentRecordID(): string | number {
  const id = record.value?.id
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
    return stringSubmitValue(field, value, errors)
  }

  if (field.studio?.editor === 'select' && (value === undefined || value === null || value === '') && field.required) {
    errors[field.name] = 'Select a value.'
    return { skip: true }
  }

  switch (field['value-kind']) {
    case 'secret': {
      const result = secretSubmitValue(value, field.required, !isNew.value)
      if (result.error) errors[field.name] = result.error
      return result
    }
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
      return stringSubmitValue(field, value, errors)
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
    if (isHiddenCollectionField(field)) {
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
  if (field.type === 'secret') {
    const result = secretSubmitValue(value, field.required, existing)
    if (result.error) setCollectionError(errors, parentField, rowIndex, `${recordFieldLabel(field)} is required.`)
    return result
  }
  if (isBlankValue(value)) {
    if (field.required) {
      setCollectionError(errors, parentField, rowIndex, `${recordFieldLabel(field)} is required.`)
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
        setCollectionError(errors, parentField, rowIndex, `${recordFieldLabel(field)} must be an integer.`)
        return { skip: true }
      }
      return { value: number }
    }
    case 'number': {
      const number = Number(value)
      if (!Number.isFinite(number)) {
        setCollectionError(errors, parentField, rowIndex, `${recordFieldLabel(field)} must be a number.`)
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
        setCollectionError(errors, parentField, rowIndex, `${recordFieldLabel(field)} must be valid JSON.`)
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

function stringSubmitValue(field: MetadataField, value: unknown, errors: Record<string, string>): ConvertedValue {
  const text = value === null || value === undefined ? '' : String(value)
  if (text === '' && field.required) {
    errors[field.name] = 'Enter a value.'
    return { skip: true }
  }

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

function isBlankValue(value: unknown): boolean {
  return value === null || value === undefined || value === ''
}

function isRecordData(value: unknown): value is RecordData {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function cloneCollectionRows(value: unknown[]): RecordData[] {
  return value.filter(isRecordData).map((row) => ({ ...row }))
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

</script>

<template>
  <section class="studio-page record-form-page" :aria-label="entityLabel">
    <PageHeader
      :show-title="false"
      :system="isSystem"
      :actions="actions"
      :pin-target="pinTarget"
    />

    <div class="record-form-page__body">
      <div v-if="loading" class="studio-page-state">
        <Spinner size="sm" label="Loading" />
        <p>Loading</p>
      </div>

      <ErrorState
        v-else-if="blockingError && !showForm"
        title="Unavailable"
        :message="blockingError"
      />

      <template v-else-if="entityMeta">
        <ErrorState
          v-if="saveError"
          title="Action failed"
          :message="saveError"
        />

        <ErrorState v-if="secretStatusQuery.isError.value" title="Secret status unavailable" message="Reload the page to try again." />
        <RecordFormRenderer
      :entity="props.entity"
      :app-name="entityMeta?.app.name ?? ''"
      :entity-key="entityMeta?.key ?? ''"
          :entity-label="entityLabel"
          :fields="fields"
          :system-fields="systemFields"
          :collections="entityMeta.collections"
          :record="record"
          :tree="entityMeta?.tree"
          :secret-status="secretStatus"
          :mode="props.mode"
          :model-value="draft"
          :field-errors="fieldErrors"
          :disabled="saving || isSystem"
          @update:model-value="updateDraft"
        />
        <RecordTimeline
          v-if="Number(timelineRecordID) > 0"
          :entries="timelineEntries"
          :loading="timelineQuery.isPending.value"
          :error="timelineError"
          :comment="commentDraft"
          :submitting="addCommentMutation.isPending.value"
          @update:comment="commentDraft = $event"
          @comment="addComment"
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

</style>
