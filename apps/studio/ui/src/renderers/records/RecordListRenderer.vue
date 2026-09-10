<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useSheetActive } from '@/features/tabs/sheet-active'
import { useInfiniteQuery, useQuery } from '@tanstack/vue-query'
import { useStorage } from '@vueuse/core'
import { ArrowDown, ArrowUp, Check, Download, Play, Settings2, X } from '@lucide/vue'
import {
  PopoverContent,
  PopoverPortal,
  PopoverRoot,
  PopoverTrigger,
} from 'reka-ui'

import { Button, Checkbox, IconButton, Input } from '@/design'
import DataTable from '@/design/organisms/DataTable.vue'
import type { DataTableRowKey, DataTableSort, DataTableState } from '@/design/types'
import type { EntityActionDefinition, MetadataField, MetadataFilterOperator } from '@/features/metadata/metadata.api'
import type { RecordListPolicy } from '@/features/platform/platform.api'
import { usePlatformConfigQuery } from '@/features/platform/platform.query'
import { useDialog } from '@/features/dialogs/use-dialog'
import { useToast } from '@/features/toasts/use-toast'
import { queryClient } from '@/app/query'
import { entityActionConfirm, entityActionDisabledReason } from '@/features/records/entity-actions'
import { recordListQueryKey } from '@/features/records/record-list.query'
import { exportRecordsCSV, listRecords, type RecordData } from '@/features/records/records.api'
import { useExecuteRecordActionMutation } from '@/features/records/record-actions.query'
import {
  buildRecordListRouteQuery,
  canonicalizeRecordListRouteQuery,
  createRecordListRouteWriter,
  isAllowedRecordPageSize,
  type RecordListFilter,
  type RecordListRouteFilterField,
  type RecordListRouteState,
  recordListRouteQueriesEqual,
} from '@/features/records/query'
import PageToolbar from '@/shell/PageToolbar.vue'
import { statusForError, storeError } from '@/stores/status'
import { buildRecordListColumns } from './columns'
import CSVImportPanel from './CSVImportPanel.vue'
import FilterFieldPicker from './FilterFieldPicker.vue'
import FilterValueInput from './FilterValueInput.vue'
import SavedFiltersMenu from '@/features/saved-filters/SavedFiltersMenu.vue'
import { savedFilterRequest, type SavedFilter } from '@/features/saved-filters/saved-filters.api'
import { validateFilters } from '@/features/records/filter-values'
import { usePageCommands } from '@/features/commands/context'
import { usePreferencesStore } from '@/features/preferences/preferences.store'
import { useAuthStore } from '@/stores/auth.store'

const props = defineProps<{
  entity: string
  entityLabel: string
  fields: MetadataField[]
  systemFields?: MetadataField[]
  readOnly?: boolean
  actions?: EntityActionDefinition[]
  appName: string
  entityKey: string
  viewKey?: string
}>()

const emit = defineEmits<{
  'create-record': []
  'open-record': [row: Record<string, unknown>]
}>()

const route = useRoute()
const router = useRouter()
const sheetActive = useSheetActive()
const platformConfigQuery = usePlatformConfigQuery()
const preferences = usePreferencesStore()
const auth = useAuthStore()
const canonicalEntity = computed(() => `${props.appName}/${props.entityKey}`)
const hiddenColumnsPreference = computed(() => `studio.records.${props.appName}.${props.entityKey}.hidden-columns`)
const ID_SEARCH_DEBOUNCE_MS = 500
const FILTER_ROUTE_REPLACE_DEBOUNCE_MS = 250
const PAGE_SIZE_STORAGE_KEY = 'dygo.studio.records.pageSize'
const DEFAULT_RECORD_LIST_SORT: DataTableSort = { key: 'created-at', direction: 'desc' }
const storedHiddenColumnKeys = useStorage<string[]>(computed(() => hiddenColumnStorageKey(props.entity)), [], undefined, {
  onError: () => {},
})
const hiddenColumnKeys = computed({
  get: () => normalizeHiddenColumnKeys(preferences.get(hiddenColumnsPreference.value, [])),
  set: (keys: string[]) => {
    preferences.set(hiddenColumnsPreference.value, normalizeHiddenColumnKeys(keys))
  },
})
const idSearch = ref('')
const idSearchControl = ref<HTMLDivElement | null>(null)
const filterPicker = ref<InstanceType<typeof FilterFieldPicker> | null>(null)
const filterTokens = ref<ActiveRecordFilter[]>([])
const listQuery = ref<RecordListRouteState>({ sort: null, filters: [] })
const storedPageSize = useStorage(PAGE_SIZE_STORAGE_KEY, 0, undefined, {
  onError: () => {},
})
const pageSize = ref(0)
const selectedRowKeys = ref<DataTableRowKey[]>([])
const viewRevision = ref(0)
const selectedAction = ref('')
const exporting = ref(false)
const importOpen = ref(false)
const actionMutation = useExecuteRecordActionMutation()
const dialog = useDialog()
const toast = useToast()
const savedFilters = useQuery({
  queryKey: computed(() => ['studio', 'saved-filters', auth.currentUser?.id, auth.sessionVersion, canonicalEntity.value]),
  queryFn: ({ signal }) => savedFilterRequest<SavedFilter[]>(`?entity=${encodeURIComponent(canonicalEntity.value)}`, 'GET', undefined, signal),
  enabled: computed(() => !!auth.currentUser),
})
const savingFilter = ref(false)
const savedFilterError = ref('')
const savedFilterRetry = ref<(() => Promise<void>) | null>(null)
const hasFilters = computed(() => !!(idSearch.value || filterTokens.value.length || listQuery.value.filters.length))
const filterContext = computed(() => Object.fromEntries(listQuery.value.filters.filter((filter) => filter.operator === 'eq').map((filter) => [filter.field, filter.value])))
usePageCommands(computed(() => [
  ...(!props.readOnly ? [{ id: 'records-new', label: `New ${props.entityLabel}`, run: createRecord }] : []),
  { id: 'records:focus-id', label: 'Focus Record ID search', run: () => { idSearchControl.value?.querySelector('input')?.focus() } },
  { id: 'records:add-filter', label: 'Add filter', run: () => { filterPicker.value?.open() } },
  ...(hasFilters.value ? [{ id: 'records-clear-filters', label: 'Clear filters', run: clearFilters }] : []),
  ...(savedFilters.data.value ?? []).map((item) => ({ id: `saved-filter-${item.id}`, label: `Apply ${item.label}`, disabled: !!item.validationError, run: () => applySavedFilter(item) })),
]))
watch([canonicalEntity, () => preferences.userID], () => {
  savedFilterError.value = ''
  savedFilterRetry.value = null
  void preferences.importMissing({
    'studio.records.page-size': storedPageSize.value || undefined,
    [hiddenColumnsPreference.value]: storedHiddenColumnKeys.value,
  })
}, { immediate: true })
watch(() => auth.sessionVersion, () => {
  clearScheduledRecordListRouteReplace()
  selectedRowKeys.value = []
  filterTokens.value = []
  syncFilterControlsFromRoute(listQuery.value.filters)
})
const viewOptionsOpen = ref(false)
let nextFilterTokenId = 1
let currentEntity = ''
let currentListQuerySignature = ''
const {
  schedule: scheduleRecordListRouteReplace,
  apply: replaceRecordListRouteNow,
  cancel: clearScheduledRecordListRouteReplace,
} = createRecordListRouteWriter(replaceRecordListRoute, FILTER_ROUTE_REPLACE_DEBOUNCE_MS)
let keepViewOptionsOpenTimer: ReturnType<typeof setTimeout> | undefined
let suppressViewOptionsClose = false

type ActiveRecordFilter = {
  id: number
  field: string
  operator: string
  value: string
  appliedValue: string
}

type OrderingOption = {
  key: string
  label: string
}

const columns = computed(() => buildRecordListColumns(props.fields, props.systemFields ?? []))
const availableActions = computed(() => (props.actions ?? []).filter((action) => action.selection !== 'collection'))
const selectedRecordIDs = computed(() => selectedRowKeys.value.flatMap((key) => {
  const id = Number(key)
  return Number.isInteger(id) && id > 0 ? [id] : []
}))
const selectedActionDefinition = computed(() => availableActions.value.find((action) => action.name === selectedAction.value) ?? null)
const recordListPolicy = computed(() => platformConfigQuery.data.value?.['record-list'] ?? null)
const pageSizeOptions = computed(() => recordListPolicy.value?.['page-sizes'] ?? [])
const filterableFields = computed(() => (
  [...props.fields, ...(props.systemFields ?? [])].filter(isFilterableField)
))
const filterableFieldByName = computed(() => new Map(filterableFields.value.map((field) => [field.name, field])))
const hiddenColumnKeySet = computed(() => new Set(hiddenColumnKeys.value.filter((key) => key !== 'name')))
const visibleColumns = computed(() => columns.value.filter((column) => (
  column.key === 'name' || !hiddenColumnKeySet.value.has(column.key)
)))
const sortableColumns = computed(() => columns.value.filter((column) => column.sortable))
const effectiveListSort = computed<DataTableSort>(() => listQuery.value.sort ?? (props.viewKey === 'tree' ? { key: 'name', direction: 'asc' } : DEFAULT_RECORD_LIST_SORT))
const orderingField = computed(() => effectiveListSort.value.key)
const orderingDirection = computed(() => effectiveListSort.value.direction)
const orderingOptions = computed<OrderingOption[]>(() => {
  const seen = new Set<string>()
  return [
    { key: DEFAULT_RECORD_LIST_SORT.key, label: systemFieldLabel(DEFAULT_RECORD_LIST_SORT.key, 'Created At') },
    ...sortableColumns.value.map((column) => ({ key: column.key, label: column.label })),
  ].filter((option) => {
    if (seen.has(option.key)) {
      return false
    }

    seen.add(option.key)
    return true
  })
})
const recordListReady = computed(() => {
  const policy = recordListPolicy.value
  return Boolean(policy && isAllowedRecordPageSize(pageSize.value, policy['page-sizes']))
})
const recordsQuery = useInfiniteQuery({
  queryKey: computed(() => [...recordListQueryKey(props.entity, {
    pageSize: pageSize.value,
    sort: effectiveListSort.value,
    filters: listQuery.value.filters,
  }), auth.currentUser?.id, auth.sessionVersion]),
  queryFn: ({ pageParam, signal }) => listRecords(props.entity, {
    limit: pageSize.value,
    offset: Number(pageParam),
    sort: effectiveListSort.value,
    filters: listQuery.value.filters,
  }, { signal }),
  initialPageParam: 0,
  getNextPageParam: (lastPage, pages) => {
    const loadedRows = pages.reduce((count, page) => count + page.data.length, 0)
    const totalRows = lastPage.meta.total ?? loadedRows
    return loadedRows < totalRows ? loadedRows : undefined
  },
  enabled: computed(() => recordListReady.value && !!auth.currentUser && (!props.viewKey || props.viewKey === 'list')),
})
const recordPages = computed(() => recordsQuery.data.value?.pages ?? [])
const rows = computed<RecordData[]>(() => recordPages.value.flatMap((page) => page.data))
const selectedRecords = computed(() => {
  const ids = new Set(selectedRecordIDs.value)
  return rows.value.filter((row) => {
    if (!row) {
      return false
    }
    const id = Number(row.id)
    return Number.isInteger(id) && id > 0 && ids.has(id)
  })
})
const canRunSelectedAction = computed(() => {
  const action = selectedActionDefinition.value
  if (!action || selectedRecordIDs.value.length === 0 || actionMutation.isPending.value) return false
  if (!(action.selection === 'selection' || selectedRecordIDs.value.length === 1)) return false
  return selectedRecords.value.every((record) => !entityActionDisabledReason(props.entityKey, action.name, record))
})
const totalRows = computed(() => recordPages.value.at(-1)?.meta.total ?? rows.value.length)
const platformConfigError = computed(() => (
  platformConfigQuery.error.value
    ? storeError(platformConfigQuery.error.value, 'Studio could not load record list settings.')
    : null
))
const queryError = computed(() => (
  recordsQuery.error.value
    ? storeError(recordsQuery.error.value, 'Studio could not load records.')
    : null
))
const tableError = computed(() => platformConfigError.value ?? queryError.value)
const recordStatus = computed<DataTableState>(() => {
  if (platformConfigError.value) {
    return statusForError(platformConfigError.value)
  }

  if (!recordListReady.value || recordsQuery.isPending.value) {
    return 'loading'
  }

  if (queryError.value && rows.value.length === 0) {
    return statusForError(queryError.value)
  }

  if (rows.value.length === 0) {
    return 'empty'
  }

  return 'ready'
})
const loading = computed(() => recordStatus.value === 'loading')
const loadingMore = computed(() => recordsQuery.isFetchingNextPage.value)
const error = computed(() => tableError.value?.message ?? '')
const footerError = computed(() => (
  recordsQuery.isFetchNextPageError.value ? queryError.value?.message ?? '' : ''
))
const tableStateTitle = computed(() => {
  switch (recordStatus.value) {
    case 'loading':
      return `Loading ${props.entityLabel} records`
    case 'empty':
      return `No ${props.entityLabel} records exist.`
    case 'forbidden':
      return `You cannot view ${props.entityLabel} records.`
    case 'unauthenticated':
      return 'Sign in to view records.'
    case 'error':
      return `${props.entityLabel} records could not load.`
    case 'ready':
    default:
      return ''
  }
})
const tableStateMessage = computed(() => {
  switch (recordStatus.value) {
    case 'forbidden':
    case 'unauthenticated':
    case 'error':
      return error.value
    case 'loading':
    case 'ready':
    default:
      return ''
  }
})
const hasMore = computed(() => (
  recordListReady.value && recordsQuery.hasNextPage.value && !platformConfigError.value
))
const showToolbar = computed(() => (
  !!props.viewKey || rows.value.length > 0 || listQuery.value.filters.length > 0 || idSearch.value !== '' || filterTokens.value.length > 0 || !props.readOnly
))
watch(() => props.viewKey, () => {
  clearScheduledRecordListRouteReplace()
  syncFilterControlsFromRoute(listQuery.value.filters)
  selectedRowKeys.value = []
  viewRevision.value++
})
watch(
  [recordListPolicy, () => preferences.get('studio.records.page-size', 0)],
  ([policy]) => {
    if (!policy) {
      pageSize.value = 0
      return
    }

    const nextPageSize = readStoredPageSize(policy)
    if (pageSize.value !== nextPageSize) {
      pageSize.value = nextPageSize
      selectedRowKeys.value = []
    }
  },
  { immediate: true },
)
watch(
  () => [
    props.entity,
    route.query,
    sheetActive.value,
    filterableFields.value.map((field) => `${field.name}:${field.filter?.operators?.map((operator) => operator.key).join(',') ?? ''}`).join('|'),
    columns.value.map((column) => `${column.key}:${column.sortable ? '1' : '0'}`).join('|'),
  ] as const,
  () => {
    if (!sheetActive.value) return
    if (currentEntity !== props.entity) {
      clearScheduledRecordListRouteReplace()
      selectedRowKeys.value = []
      filterTokens.value = []
      currentEntity = props.entity
    }

    const query = routeRecordListQuery()
    const listQuerySignature = recordListStateSignature(query)
    if (currentListQuerySignature !== '' && currentListQuerySignature !== listQuerySignature) {
      clearScheduledRecordListRouteReplace()
      selectedRowKeys.value = []
    }
    currentListQuerySignature = listQuerySignature

    listQuery.value = {
      sort: query.sort,
      filters: query.filters.map((filter) => ({ ...filter })),
    }
    syncFilterControlsFromRoute(query.filters)
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  clearScheduledRecordListRouteReplace()
  clearKeepViewOptionsOpenTimer()
})

function updatePageSize(value: number) {
  const policy = recordListPolicy.value
  if (!policy || !isAllowedRecordPageSize(value, policy['page-sizes'])) {
    return
  }

  pageSize.value = value
  preferences.set('studio.records.page-size', value)
  selectedRowKeys.value = []
}

function updateSelectedRowKeys(value: DataTableRowKey[]) {
  selectedRowKeys.value = value
}

function updateSort(value: DataTableSort | null) {
  replaceRecordListRouteNow(appliedRecordFilters(), defaultRecordListSortEquals(value) ? null : value)
}

function updateViewOptionsOpen(value: boolean) {
  if (!value && suppressViewOptionsClose) {
    viewOptionsOpen.value = true
    return
  }

  viewOptionsOpen.value = value
}

function updateOrderingField(value: string) {
  keepViewOptionsOpen()
  if (value === '') {
    updateSort(null)
    return
  }

  if (value === DEFAULT_RECORD_LIST_SORT.key) {
    updateSort(DEFAULT_RECORD_LIST_SORT)
    return
  }

  updateSort({ key: value, direction: listQuery.value.sort?.direction ?? 'asc' })
}

function toggleOrderingDirection() {
  keepViewOptionsOpen()
  const sort = effectiveListSort.value

  updateSort({
    key: sort.key,
    direction: sort.direction === 'asc' ? 'desc' : 'asc',
  })
}

function selectFilterField(key: string) {
  const field = filterableFieldByName.value.get(key)
  const operator = field?.filter?.operators?.[0]?.key
  if (!field || !operator) {
    return
  }

  const value = defaultFilterValue(field, operator)
  const appliesImmediately = filterAppliesImmediately(field, operator)
  filterTokens.value = [
    ...filterTokens.value,
    {
      id: nextFilterTokenId,
      field: key,
      operator,
      value,
      appliedValue: appliesImmediately ? normalizeFilterValue(value) : '',
    },
  ]
  nextFilterTokenId += 1
  if (appliesImmediately) {
    scheduleRecordListRouteReplace(appliedRecordFilters(), listQuery.value.sort)
  }
}

function updateIDSearch(value: string) {
  idSearch.value = value
  if (value.trim() === '') {
    applyIDSearch()
    return
  }

  scheduleRecordListRouteReplace(appliedRecordFilters(), listQuery.value.sort, ID_SEARCH_DEBOUNCE_MS)
}

function applyIDSearch() {
  replaceRecordListRouteNow(appliedRecordFilters(), listQuery.value.sort)
}

function updateFilterOperator(id: number, operator: string) {
  filterTokens.value = filterTokens.value.map((filter) => {
    if (filter.id !== id) {
      return filter
    }
    const field = filterableFieldByName.value.get(filter.field)
    const value = defaultFilterValue(field, operator)
    const appliesImmediately = filterAppliesImmediately(field, operator)
    return {
      ...filter,
      operator,
      value,
      appliedValue: appliesImmediately ? normalizeFilterValue(value) : '',
    }
  })
  scheduleRecordListRouteReplace(appliedRecordFilters(), listQuery.value.sort)
}

function updateFilterValue(id: number, value: string, applyImmediately = false) {
  filterTokens.value = filterTokens.value.map((filter) => (
    filter.id === id ? { ...filter, value } : filter
  ))

  if (applyImmediately) {
    applyFilter(id, { debounce: true })
  }
}

function applyFilter(id: number, options: { debounce?: boolean } = {}) {
  const filter = filterTokens.value.find((candidate) => candidate.id === id)
  if (!filter) {
    return
  }
  const problem = validateFilters([{ field: filter.field, operator: filter.operator, value: filter.value }], filterableFields.value)
  if (problem) {
    toast.error('Filter needs attention', problem)
    return
  }

  if (filterHasValue(filter) && normalizeFilterValue(filter.value) === '') {
    filterTokens.value = filterTokens.value.filter((candidate) => candidate.id !== id)
  } else {
    filterTokens.value = filterTokens.value.map((candidate) => (
      candidate.id === id
        ? { ...candidate, appliedValue: normalizeFilterValue(candidate.value) }
        : candidate
    ))
  }

  if (options.debounce) {
    scheduleRecordListRouteReplace(appliedRecordFilters(), listQuery.value.sort)
    return
  }

  replaceRecordListRouteNow(appliedRecordFilters(), listQuery.value.sort)
}

function removeFilter(id: number) {
  filterTokens.value = filterTokens.value.filter((filter) => filter.id !== id)
  replaceRecordListRouteNow(appliedRecordFilters(), listQuery.value.sort)
}

function clearFilters() {
  viewRevision.value++
  clearScheduledRecordListRouteReplace()
  idSearch.value = ''
  filterTokens.value = []
  selectedRowKeys.value = []
  listQuery.value = { sort: listQuery.value.sort, filters: [] }
  replaceRecordListRouteNow([], listQuery.value.sort)
  void queryClient.resetQueries({ queryKey: [...recordListQueryKey(props.entity, { pageSize: pageSize.value, sort: effectiveListSort.value, filters: [] }), auth.currentUser?.id, auth.sessionVersion], exact: true })
}

function applySavedFilter(item: SavedFilter) {
  const problem = item.validationError || (item.entity !== canonicalEntity.value ? 'This filter belongs to another Entity.' : validateFilters(item.filters, filterableFields.value))
  if (problem) {
    toast.error('Saved filter needs attention', problem)
    return
  }
  clearScheduledRecordListRouteReplace()
  viewRevision.value++
  filterTokens.value = []
  selectedRowKeys.value = []
  syncFilterControlsFromRoute(item.filters)
  replaceRecordListRouteNow(item.filters, listQuery.value.sort)
  void queryClient.resetQueries({ queryKey: [...recordListQueryKey(props.entity, { pageSize: pageSize.value, sort: effectiveListSort.value, filters: item.filters }), auth.currentUser?.id, auth.sessionVersion], exact: true })
}

async function changeSavedFilter(method: string, item?: SavedFilter, label?: string) {
  if (savingFilter.value) return
  const filters = listQuery.value.filters.map((filter) => ({ ...filter }))
  if (method === 'POST' || (method === 'PATCH' && label === undefined)) {
    const problem = validateFilters(filters, filterableFields.value)
    if (problem) { toast.error('Filter needs attention', problem); return }
  }
  const session = auth.sessionVersion
  const entity = canonicalEntity.value
  savedFilterRetry.value = () => changeSavedFilter(method, item, label)
  savingFilter.value = true
  savedFilterError.value = ''
  try {
    await savedFilterRequest(item ? `/${item.id}` : '', method, method === 'DELETE' ? undefined : method === 'POST' ? { entity, label, filters } : label === undefined ? { filters } : { label })
    await queryClient.invalidateQueries({ queryKey: ['studio', 'saved-filters'] })
    savedFilterRetry.value = null
  } catch (cause) {
    if (session === auth.sessionVersion && entity === canonicalEntity.value) savedFilterError.value = cause instanceof Error ? cause.message : 'Could not update saved filters.'
  } finally {
    savingFilter.value = false
  }
}

async function retrySavedFilter() {
  if (savedFilterRetry.value) {
    await savedFilterRetry.value()
    return
  }
  await savedFilters.refetch()
}

function showAllColumns() {
  keepViewOptionsOpen()
  hiddenColumnKeys.value = []
}

function updateColumnVisibility(key: string, visible: boolean) {
  keepViewOptionsOpen()
  if (key === 'name') {
    return
  }

  const nextHiddenColumns = new Set(hiddenColumnKeySet.value)
  if (visible) {
    nextHiddenColumns.delete(key)
  } else {
    nextHiddenColumns.add(key)
  }

  hiddenColumnKeys.value = Array.from(nextHiddenColumns).sort()

  if (!visible && listQuery.value.sort?.key === key) {
    updateSort(null)
  }
}

function hiddenColumnStorageKey(entity: string): string {
  return `dygo.studio.records.hiddenColumns.${entity}`
}

function normalizeHiddenColumnKeys(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return []
  }

  return value.filter((key): key is string => typeof key === 'string' && key !== 'name')
}

function defaultPageSize(policy: RecordListPolicy): number {
  return policy['page-sizes'][0] ?? policy['default-limit']
}

function readStoredPageSize(policy: RecordListPolicy): number {
  const value = Number(preferences.get('studio.records.page-size', 0))
  if (!isAllowedRecordPageSize(value, policy['page-sizes'])) {
    return defaultPageSize(policy)
  }

  return value
}

function createRecord() {
  if (props.readOnly) {
    return
  }
  emit('create-record')
}

function isFilterableField(field: MetadataField): boolean {
  if (!field.listable || field['write-only']) {
    return false
  }

  if (field.name === 'id' || field.name === 'name') {
    return false
  }

  return (field.filter?.operators?.length ?? 0) > 0
}

function filterFieldLabel(field: MetadataField): string {
  return field.name === 'name' ? 'ID' : field.label || field.name
}

function systemFieldLabel(name: string, fallback: string): string {
  return props.systemFields?.find((field) => field.name === name)?.label || fallback
}

function filterFieldForToken(filter: ActiveRecordFilter): MetadataField | null {
  return filterableFieldByName.value.get(filter.field) ?? null
}

function filterLabel(filter: ActiveRecordFilter): string {
  const field = filterFieldForToken(filter)
  return field ? filterFieldLabel(field) : 'Field'
}

function filterOperators(filter: ActiveRecordFilter): MetadataFilterOperator[] {
  return filterFieldForToken(filter)?.filter?.operators ?? []
}

function filterOperatorArity(filter: ActiveRecordFilter): MetadataFilterOperator['arity'] {
  return filterOperators(filter).find((operator) => operator.key === filter.operator)?.arity ?? 'one'
}

function filterHasValue(filter: ActiveRecordFilter): boolean {
  return filterOperatorArity(filter) !== 'none'
}

function filterTokenDirty(filter: ActiveRecordFilter): boolean {
  return filterHasValue(filter) && normalizeFilterValue(filter.value) !== filter.appliedValue
}

function defaultFilterValue(field: MetadataField | undefined, operator: string): string {
  if (filterOperatorArityForField(field, operator) === 'none') {
    return ''
  }
  if (field?.type === 'boolean') {
    return 'true'
  }
  return ''
}

function filterAppliesImmediately(field: MetadataField | undefined, operator: string): boolean {
  return filterOperatorArityForField(field, operator) === 'none' || field?.type === 'boolean'
}

function filterOperatorArityForField(field: MetadataField | undefined, operator: string): MetadataFilterOperator['arity'] {
  return field?.filter?.operators?.find((candidate) => candidate.key === operator)?.arity ?? 'one'
}

function normalizeFilterValue(value: string): string {
  return value.trim()
}

function keepViewOptionsOpen() {
  clearKeepViewOptionsOpenTimer()
  suppressViewOptionsClose = true
  viewOptionsOpen.value = true
  keepViewOptionsOpenTimer = setTimeout(() => {
    suppressViewOptionsClose = false
    viewOptionsOpen.value = true
    keepViewOptionsOpenTimer = undefined
  }, 350)
}

function clearKeepViewOptionsOpenTimer() {
  if (keepViewOptionsOpenTimer === undefined) {
    return
  }

  clearTimeout(keepViewOptionsOpenTimer)
  keepViewOptionsOpenTimer = undefined
  suppressViewOptionsClose = false
}

function replaceRecordListRoute(filters: RecordListFilter[], sort: DataTableSort | null) {
  if (!sheetActive.value) return
  const nextQuery = buildRecordListRouteQuery({ filters, sort })
  if (recordListRouteQueriesEqual(route.query, nextQuery)) {
    return
  }

  void router.replace({ query: nextQuery })
}

function appliedRecordFilters(): RecordListFilter[] {
  const filters: RecordListFilter[] = []
  const search = idSearch.value.trim()
  if (search !== '') {
    filters.push({ field: 'name', operator: 'contains', value: search })
  }

  filterTokens.value.forEach((filter) => {
    const field = filterFieldForToken(filter)
    if (!field || !field.filter?.operators?.some((operator) => operator.key === filter.operator)) {
      return
    }

    if (filterOperatorArity(filter) === 'none') {
      filters.push({ field: filter.field, operator: filter.operator })
      return
    }

    const value = filter.appliedValue
    if (value !== '') {
      filters.push({ field: filter.field, operator: filter.operator, value })
    }
  })

  return filters
}

function routeRecordListQuery(): RecordListRouteState {
  const canonical = canonicalizeRecordListRouteQuery(route.query, recordListRouteSchema())
  if (canonical.changed && sheetActive.value) {
    void router.replace({ query: canonical.query })
  }

  return canonical.state
}

function recordListStateSignature(state: RecordListRouteState): string {
  return JSON.stringify(buildRecordListRouteQuery(state))
}

function recordListRouteSchema() {
  const filterFields: RecordListRouteFilterField[] = [
    { field: 'name', operators: [{ key: 'contains', arity: 'one' }] },
    ...filterableFields.value.map((field): RecordListRouteFilterField => ({
      field: field.name,
      operators: field.filter?.operators ?? [],
    })),
  ]

  return {
    sortableFields: orderingOptions.value.map((option) => option.key),
    filterFields,
  }
}

function defaultRecordListSortEquals(sort: DataTableSort | null): boolean {
  return sort?.key === DEFAULT_RECORD_LIST_SORT.key && sort.direction === DEFAULT_RECORD_LIST_SORT.direction
}

function syncFilterControlsFromRoute(filters: RecordListFilter[]) {
  const idFilter = filters.find((filter) => filter.field === 'name' && filter.operator === 'contains')
  idSearch.value = idFilter?.value ?? ''
  const nextTokens = filters
    .filter((filter) => !(filter.field === 'name' && filter.operator === 'contains'))
    .map((filter) => ({
      field: filter.field,
      operator: filter.operator,
      value: filter.value ?? '',
      appliedValue: filter.value ?? '',
    }))

  filterTokens.value = mergeRouteFilterTokens(nextTokens)
}

function mergeRouteFilterTokens(filters: Array<Omit<ActiveRecordFilter, 'id'>>): ActiveRecordFilter[] {
  const existing = new Map<string, ActiveRecordFilter[]>()
  filterTokens.value.forEach((filter) => {
    const key = filterTokenKey(filter)
    existing.set(key, [...(existing.get(key) ?? []), filter])
  })

  const reusedIDs = new Set<number>()
  const routeTokens = filters.map((filter) => {
    const key = filterTokenKey(filter)
    const reusable = existing.get(key)?.shift()
    if (reusable) {
      reusedIDs.add(reusable.id)
    }
    return {
      id: reusable?.id ?? nextFilterTokenId++,
      ...filter,
    }
  })

  const draftTokens = filterTokens.value.filter((filter) => (
    !reusedIDs.has(filter.id) && filterHasValue(filter) && (filterTokenDirty(filter) || filter.appliedValue === '')
  ))

  return [...routeTokens, ...draftTokens]
}

function filterTokenKey(filter: Omit<ActiveRecordFilter, 'id'>): string {
  return `${filter.field}\u0000${filter.operator}`
}

async function runSelectedAction() {
  const action = selectedActionDefinition.value
  const records = selectedRecordIDs.value
  if (!action || !canRunSelectedAction.value) return

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
    if (decision !== 'confirm') return
  }

  try {
    await actionMutation.mutateAsync({ entity: props.entity, action: action.name, records })
    selectedRowKeys.value = []
    await queryClient.invalidateQueries({ queryKey: recordListQueryKey(props.entity, { pageSize: pageSize.value, sort: effectiveListSort.value, filters: listQuery.value.filters }) })
    toast.success(`${action.label} complete`)
  } catch (error) {
    toast.error(`${action.label} failed`, error instanceof Error ? error.message : 'The action could not be completed.')
  }
}

async function exportCSV() {
  if (exporting.value) return
  exporting.value = true
  try {
    const csv = await exportRecordsCSV(props.entity, { limit: 1, offset: 0, sort: effectiveListSort.value, filters: listQuery.value.filters })
    const url = URL.createObjectURL(csv)
    const link = document.createElement('a')
    link.href = url
    link.download = `${props.entity}-records.csv`
    link.click()
    URL.revokeObjectURL(url)
    toast.success('CSV export ready')
  } catch (error) {
    toast.error('CSV export failed', error instanceof Error ? error.message : 'The records could not be exported.')
  } finally {
    exporting.value = false
  }
}

</script>

<template>
  <section class="record-list-renderer" aria-label="Record list view">
    <PageToolbar v-if="showToolbar">
      <template #left>
        <slot name="view-selector" />
        <div ref="idSearchControl" class="record-list-renderer__name-search">
          <Input
            :model-value="idSearch"
            type="search"
            placeholder="ID"
            aria-label="Filter records by ID"
            @update:model-value="updateIDSearch"
            @keydown.enter.prevent="applyIDSearch"
          />
        </div>

        <div class="record-list-renderer__filter-controls">
          <div
            v-for="filter in filterTokens"
            :key="filter.id"
            class="record-list-renderer__filter-token"
            :class="{ 'record-list-renderer__filter-token--dirty': filterTokenDirty(filter) }"
            aria-label="Active filter"
          >
            <span class="record-list-renderer__filter-segment record-list-renderer__filter-segment--field">
              {{ filterLabel(filter) }}
            </span>
            <select
              class="record-list-renderer__filter-segment record-list-renderer__filter-segment--operator"
              :value="filter.operator"
              :aria-label="`${filterLabel(filter)} operator`"
              @change="updateFilterOperator(filter.id, ($event.target as HTMLSelectElement).value)"
            >
              <option
                v-for="operator in filterOperators(filter)"
                :key="operator.key"
                :value="operator.key"
              >
                {{ operator.label }}
              </option>
            </select>
            <FilterValueInput v-if="filterFieldForToken(filter)" :field="filterFieldForToken(filter)!" :operator="filter.operator" :model-value="filter.value" :current-values="filterContext" :app-name="appName" @update:model-value="updateFilterValue(filter.id, $event)" @apply="applyFilter(filter.id)" />
            <button
              v-if="filterTokenDirty(filter)"
              class="record-list-renderer__filter-apply"
              type="button"
              aria-label="Apply filter"
              @click="applyFilter(filter.id)"
            >
              <Check :size="13" :stroke-width="2" aria-hidden="true" />
            </button>
            <button
              class="record-list-renderer__filter-remove"
              type="button"
              aria-label="Remove filter"
              @click="removeFilter(filter.id)"
            >
              <X :size="13" :stroke-width="2" aria-hidden="true" />
            </button>
          </div>

          <FilterFieldPicker ref="filterPicker" :fields="filterableFields" @select="selectFilterField" />
          <Button v-if="hasFilters" variant="ghost" size="sm" @click="clearFilters">Clear all</Button>
          <SavedFiltersMenu :items="savedFilters.data.value ?? []" :busy="savingFilter || savedFilters.isLoading.value" :error="savedFilterError || savedFilters.error.value?.message || ''" @apply="applySavedFilter" @create="changeSavedFilter('POST', undefined, $event)" @rename="(item, label) => changeSavedFilter('PATCH', item, label)" @replace="changeSavedFilter('PATCH', $event)" @delete="changeSavedFilter('DELETE', $event)" @retry="retrySavedFilter" />
        </div>
      </template>

      <template #right>
        <select
          v-if="availableActions.length > 0 && (!viewKey || viewKey === 'list')"
          v-model="selectedAction"
          class="record-list-renderer__action-select"
          aria-label="Entity action"
        >
          <option value="">Action</option>
          <option v-for="action in availableActions" :key="action.name" :value="action.name">
            {{ action.label }}
          </option>
        </select>
        <Button
          v-if="availableActions.length > 0 && (!viewKey || viewKey === 'list')"
          variant="secondary"
          size="sm"
          :disabled="!canRunSelectedAction"
          :loading="actionMutation.isPending.value"
          @click="runSelectedAction"
        >
          <Play :size="13" :stroke-width="1.9" aria-hidden="true" />
          Run
        </Button>
        <Button variant="ghost" size="sm" :disabled="exporting || !recordListReady" :loading="exporting" @click="exportCSV">
          <Download :size="13" :stroke-width="1.9" aria-hidden="true" />
          Export CSV
        </Button>
        <Button v-if="!readOnly" variant="ghost" size="sm" @click="importOpen = !importOpen">
          Import CSV
        </Button>
        <PopoverRoot
          :open="viewOptionsOpen"
          @update:open="updateViewOptionsOpen"
        >
          <PopoverTrigger as-child>
            <IconButton label="View options" type="button" variant="secondary">
              <Settings2 :size="14" :stroke-width="1.8" aria-hidden="true" />
            </IconButton>
          </PopoverTrigger>

          <PopoverPortal>
            <PopoverContent
              class="record-list-renderer__view-options"
              align="end"
              :side-offset="6"
            >
              <section class="record-list-renderer__view-options-section">
                <div class="record-list-renderer__view-options-row">
                  <span class="record-list-renderer__view-options-label">Ordering</span>
                  <div class="record-list-renderer__ordering-controls">
                    <select
                      class="record-list-renderer__ordering-field"
                      :value="orderingField"
                      aria-label="Ordering field"
                      @change="updateOrderingField(($event.target as HTMLSelectElement).value)"
                    >
                      <option value="">Field</option>
                      <option
                        v-for="option in orderingOptions"
                        :key="option.key"
                        :value="option.key"
                      >
                        {{ option.label }}
                      </option>
                    </select>
                    <button
                      class="record-list-renderer__ordering-direction"
                      type="button"
                      :aria-label="orderingDirection === 'asc' ? 'Ascending' : 'Descending'"
                      @click="toggleOrderingDirection"
                    >
                      <ArrowUp
                        v-if="orderingDirection === 'asc'"
                        :size="14"
                        :stroke-width="1.9"
                        aria-hidden="true"
                      />
                      <ArrowDown
                        v-else
                        :size="14"
                        :stroke-width="1.9"
                        aria-hidden="true"
                      />
                    </button>
                  </div>
                </div>
              </section>

              <section v-if="!viewKey || viewKey === 'list'" class="record-list-renderer__view-options-section">
                <div class="record-list-renderer__view-options-heading">Display properties</div>
                <div class="record-list-renderer__property-list">
                  <label
                    v-for="column in columns"
                    :key="column.key"
                    class="record-list-renderer__property-row"
                    :class="{ 'record-list-renderer__property-row--disabled': column.key === 'name' }"
                  >
                    <Checkbox
                      :model-value="column.key === 'name' || !hiddenColumnKeySet.has(column.key)"
                      :disabled="column.key === 'name'"
                      @update:model-value="(visible) => updateColumnVisibility(column.key, visible)"
                    />
                    <span>{{ column.label }}</span>
                  </label>
                </div>
                <button
                  class="record-list-renderer__show-all-properties"
                  type="button"
                  :disabled="hiddenColumnKeySet.size === 0"
                  @click="showAllColumns"
                >
                  Show all properties
                </button>
              </section>
            </PopoverContent>
          </PopoverPortal>
        </PopoverRoot>

      </template>
    </PageToolbar>

    <CSVImportPanel
      v-if="importOpen"
      :entity="entity"
      :app-name="appName"
      :entity-key="entityKey"
      :fields="fields"
      @close="importOpen = false"
      @imported="() => { recordsQuery.refetch(); viewRevision++ }"
    />

    <slot name="records" :query="listQuery" :page-size="pageSize" :revision="viewRevision">
    <DataTable
      :columns="visibleColumns"
      :rows="rows"
      :state="recordStatus"
      :state-title="tableStateTitle"
      :state-message="tableStateMessage"
      :loading="loading"
      :loading-more="loadingMore"
      :error="error"
      :footer-error="footerError"
      :page-size="pageSize"
      :page-size-options="pageSizeOptions"
      :total-rows="totalRows"
      :has-more="hasMore"
      :sort="listQuery.sort"
      selectable
      :selected-row-keys="selectedRowKeys"
      :empty-action-label="readOnly ? '' : 'Add first record'"
      row-activatable
      @update:page-size="updatePageSize"
      @update:selected-row-keys="updateSelectedRowKeys"
      @update:sort="updateSort"
      @row-activate="(row) => emit('open-record', row)"
      @load-more="recordsQuery.fetchNextPage()"
      @empty-action="createRecord"
    />
    </slot>
  </section>
</template>

<style scoped>
.record-list-renderer {
  display: flex;
  min-width: 0;
  min-height: 0;
  flex-direction: column;
  margin: 0 calc(var(--studio-page-padding) * -1) calc(var(--studio-page-padding) * -1);
}

.record-list-renderer :deep(.data-table) {
  flex: 1 1 auto;
}

.record-list-renderer__name-search {
  width: 180px;
  flex: 0 0 180px;
  min-width: 0;
}

.record-list-renderer__action-select {
  min-height: var(--studio-control-height-sm);
  max-width: 180px;
  border: 1px solid var(--studio-border);
  border-radius: var(--studio-radius-control);
  background: var(--studio-control-bg);
  color: var(--studio-text);
  padding: 0 8px;
  font-size: 12px;
}

.record-list-renderer__filter-controls {
  display: flex;
  flex: 1 1 240px;
  flex-wrap: wrap;
  min-width: 0;
  align-items: center;
  gap: 6px;
}

.record-list-renderer__filter-token {
  display: inline-flex;
  max-width: 100%;
  height: var(--studio-control-height-xs);
  min-width: 0;
  align-items: stretch;
  overflow: hidden;
  border: 1px solid var(--studio-border);
  border-radius: var(--studio-radius-control);
  background: var(--studio-control-bg);
  box-shadow: var(--studio-shadow-control);
}

.record-list-renderer__filter-segment,
.record-list-renderer__filter-apply,
.record-list-renderer__filter-remove {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  height: 100%;
  border: 0;
  border-right: 1px solid var(--studio-border);
  background: transparent;
  color: var(--studio-text-muted);
  font: inherit;
  font-size: 13px;
  line-height: 1;
}

.record-list-renderer__filter-token--dirty {
  border-color: var(--studio-border-strong);
}

.record-list-renderer__filter-segment {
  max-width: 120px;
  padding: 0 8px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

select.record-list-renderer__filter-segment,
input.record-list-renderer__filter-segment {
  border-radius: 0;
  outline: none;
}

select.record-list-renderer__filter-segment {
  cursor: pointer;
}

.record-list-renderer__filter-segment--field {
  color: var(--studio-text);
  font-weight: 500;
}

.record-list-renderer__filter-segment--operator {
  max-width: 150px;
  color: var(--studio-text-subtle);
}

.record-list-renderer__filter-segment--value {
  min-width: 78px;
  background: var(--studio-surface);
  color: var(--studio-text);
}

.record-list-renderer__filter-apply,
.record-list-renderer__filter-remove {
  width: var(--studio-control-height-xs);
  flex: 0 0 auto;
  justify-content: center;
}

.record-list-renderer__filter-apply {
  color: var(--studio-text);
}

.record-list-renderer__filter-remove {
  border-right: 0;
}

.record-list-renderer__filter-segment:hover,
.record-list-renderer__filter-apply:hover,
.record-list-renderer__filter-remove:hover {
  background: var(--studio-surface-raised);
  color: var(--studio-text);
}

.record-list-renderer__filter-segment:focus-visible,
.record-list-renderer__filter-apply:focus-visible,
.record-list-renderer__filter-remove:focus-visible {
  outline: 2px solid var(--studio-focus);
  outline-offset: -2px;
}

:global(.record-list-renderer__view-options) {
  z-index: 50;
  width: min(360px, calc(100vw - 24px));
  max-height: min(520px, calc(100vh - 40px));
  overflow-y: auto;
  border: 1px solid var(--studio-border);
  border-radius: var(--studio-radius-control);
  background: var(--studio-surface);
  box-shadow: var(--studio-shadow-sheet);
  color: var(--studio-text);
  outline: none;
  padding: 12px;
}

:global(.record-list-renderer__view-options-section + .record-list-renderer__view-options-section) {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--studio-border);
}

:global(.record-list-renderer__view-options-row) {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 16px;
  align-items: center;
}

:global(.record-list-renderer__view-options-label),
:global(.record-list-renderer__view-options-heading) {
  color: var(--studio-text-muted);
  font-size: 13px;
  font-weight: 650;
  line-height: 1;
}

:global(.record-list-renderer__view-options-heading) {
  margin-bottom: 8px;
}

:global(.record-list-renderer__ordering-controls) {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: 6px;
}

:global(.record-list-renderer__ordering-field) {
  width: 150px;
  height: var(--studio-control-height-xs);
  border: 1px solid var(--studio-border);
  border-radius: var(--studio-radius-control);
  background: var(--studio-control-bg);
  box-shadow: var(--studio-shadow-control);
  color: var(--studio-text);
  font: inherit;
  font-size: 13px;
  line-height: 1;
  padding: 0 8px;
}

:global(.record-list-renderer__ordering-field:focus-visible) {
  outline: 2px solid var(--studio-focus);
  outline-offset: 2px;
}

:global(.record-list-renderer__ordering-direction) {
  display: inline-flex;
  width: var(--studio-control-height-xs);
  height: var(--studio-control-height-xs);
  align-items: center;
  justify-content: center;
  border: 1px solid var(--studio-border);
  border-radius: var(--studio-radius-control);
  background: var(--studio-control-bg);
  box-shadow: var(--studio-shadow-control);
  color: var(--studio-text-muted);
}

:global(.record-list-renderer__ordering-direction:hover:not(:disabled)) {
  border-color: var(--studio-border-strong);
  background: var(--studio-control-bg-hover);
  color: var(--studio-text);
}

:global(.record-list-renderer__ordering-direction:focus-visible) {
  outline: 2px solid var(--studio-focus);
  outline-offset: 2px;
}

:global(.record-list-renderer__ordering-direction:disabled) {
  opacity: 0.58;
}

:global(.record-list-renderer__property-list) {
  display: grid;
  gap: 2px;
}

:global(.record-list-renderer__property-row) {
  display: flex;
  min-height: 30px;
  align-items: center;
  gap: 8px;
  border-radius: 5px;
  color: var(--studio-text-muted);
  font-size: 13px;
  font-weight: 500;
  line-height: 1;
  padding: 0 4px;
}

:global(.record-list-renderer__property-row:hover:not(.record-list-renderer__property-row--disabled)) {
  background: var(--studio-surface-raised);
  color: var(--studio-text);
}

:global(.record-list-renderer__property-row--disabled) {
  color: var(--studio-text-subtle);
}

:global(.record-list-renderer__show-all-properties) {
  min-height: 28px;
  margin-top: 8px;
  border: 0;
  border-radius: 5px;
  background: transparent;
  color: var(--studio-text-muted);
  font-size: 13px;
  font-weight: 500;
  line-height: 1;
  padding: 0 8px;
}

:global(.record-list-renderer__show-all-properties:hover:not(:disabled)) {
  background: var(--studio-surface-raised);
  color: var(--studio-text);
}

:global(.record-list-renderer__show-all-properties:focus-visible) {
  outline: 2px solid var(--studio-focus);
  outline-offset: 2px;
}

:global(.record-list-renderer__show-all-properties:disabled) {
  color: var(--studio-text-subtle);
  opacity: 0.62;
}
</style>
