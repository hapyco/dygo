<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowUpDown, Check, FunnelPlus, PanelRightOpen, Settings2, X } from '@lucide/vue'

import { IconButton, Input } from '@/design'
import DataTable from '@/design/organisms/DataTable.vue'
import DropdownMenu from '@/design/primitives/DropdownMenu.vue'
import type { DataTableRowKey, DataTableSort, DataTableState, DropdownMenuItem } from '@/design/types'
import type { MetadataField, MetadataFilterOperator } from '@/features/metadata/metadata.api'
import {
  buildRecordListRouteQuery,
  parseRecordListRouteQuery,
  type RecordListFilter,
  type RecordListRouteState,
} from '@/features/records/query'
import PageToolbar from '@/shell/PageToolbar.vue'
import { usePlatformStore } from '@/stores/platform.store'
import { useRecordsStore } from '@/stores/records.store'
import { buildRecordListColumns } from './columns'

const props = defineProps<{
  entity: string
  entityLabel: string
  fields: MetadataField[]
  systemFields?: MetadataField[]
  readOnly?: boolean
}>()

const emit = defineEmits<{
  'create-record': []
  'open-record': [row: Record<string, unknown>]
}>()

const recordsStore = useRecordsStore()
const platformStore = usePlatformStore()
const route = useRoute()
const router = useRouter()
const hiddenColumnKeys = ref<string[]>([])
const idSearch = ref('')
const filterTokens = ref<ActiveRecordFilter[]>([])
let nextFilterTokenId = 1
let currentEntity = ''
let idSearchDebounce: ReturnType<typeof setTimeout> | undefined
const ID_SEARCH_DEBOUNCE_MS = 700

type ActiveRecordFilter = {
  id: number
  field: string
  operator: string
  value: string
  appliedValue: string
}

const columns = computed(() => buildRecordListColumns(props.fields, props.systemFields ?? []))
const pageSizeOptions = computed(() => platformStore.recordListPolicy?.['page-sizes'] ?? [])
const filterableFields = computed(() => (
  [...props.fields, ...(props.systemFields ?? [])].filter(isFilterableField)
))
const filterableFieldByName = computed(() => new Map(filterableFields.value.map((field) => [field.name, field])))
const filterFieldMenuItems = computed<DropdownMenuItem[]>(() => {
  if (filterableFields.value.length === 0) {
    return [{ type: 'item', key: 'empty', label: 'No filterable fields', disabled: true }]
  }

  return [
    { type: 'label', key: 'filter-fields-label', label: 'Fields' },
    ...filterableFields.value.map((field) => ({
      type: 'item' as const,
      key: field.name,
      label: filterFieldLabel(field),
    })),
  ]
})
const hiddenColumnKeySet = computed(() => new Set(hiddenColumnKeys.value.filter((key) => key !== 'name')))
const visibleColumns = computed(() => columns.value.filter((column) => (
  column.key === 'name' || !hiddenColumnKeySet.value.has(column.key)
)))
const recordState = computed(() => recordsStore.entityState(props.entity))
const loading = computed(() => recordState.value.status === 'idle' || recordState.value.status === 'loading')
const error = computed(() => recordState.value.error?.message ?? '')
const footerError = computed(() => recordState.value.loadMoreError?.message ?? '')
const tableState = computed<DataTableState>(() => {
  if (loading.value) {
    return 'loading'
  }

  if (recordState.value.status === 'forbidden') {
    return 'forbidden'
  }

  if (recordState.value.status === 'unauthenticated') {
    return 'unauthenticated'
  }

  if (recordState.value.status === 'empty') {
    return 'empty'
  }

  if (recordState.value.status === 'error' || error.value) {
    return 'error'
  }

  return 'ready'
})
const tableStateTitle = computed(() => {
  switch (tableState.value) {
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
  switch (tableState.value) {
    case 'empty':
      return 'Create the first record to start using this Entity.'
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
  recordState.value.rows.length < recordState.value.total && !recordState.value.error
))
const showToolbar = computed(() => (
  recordState.value.rows.length > 0 || recordState.value.filters.length > 0 || idSearch.value !== '' || filterTokens.value.length > 0
))
const columnMenuItems = computed<DropdownMenuItem[]>(() => [
  { type: 'item', key: 'show-all', label: 'Show all', disabled: hiddenColumnKeySet.value.size === 0 },
  { type: 'separator', key: 'columns-separator' },
  ...columns.value.map((column) => ({
    type: 'checkbox' as const,
    key: column.key,
    label: column.label,
    checked: column.key === 'name' || !hiddenColumnKeySet.value.has(column.key),
    disabled: column.key === 'name',
  })),
])

watch(
  () => [
    props.entity,
    route.query,
    filterableFields.value.map((field) => `${field.name}:${field.filter?.operators?.map((operator) => operator.key).join(',') ?? ''}`).join('|'),
    columns.value.map((column) => `${column.key}:${column.sortable ? '1' : '0'}`).join('|'),
  ] as const,
  async () => {
    if (currentEntity !== props.entity) {
      hiddenColumnKeys.value = readHiddenColumnKeys(props.entity)
      currentEntity = props.entity
    }

    const query = routeRecordListQuery()
    syncFilterControlsFromRoute(query.filters)
    await recordsStore.setListQuery(props.entity, query)
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  clearIDSearchDebounce()
})

function updatePageSize(value: number) {
  void recordsStore.setPageSize(props.entity, value)
}

function updateSelectedRowKeys(value: DataTableRowKey[]) {
  if (props.readOnly) {
    return
  }
  recordsStore.setSelectedRowKeys(props.entity, value)
}

function updateSort(value: DataTableSort | null) {
  replaceRecordListRoute(appliedRecordFilters(), value)
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
    replaceRecordListRoute(appliedRecordFilters(), recordState.value.sort)
  }
}

function updateIDSearch(value: string) {
  idSearch.value = value
  if (value.trim() === '') {
    applyIDSearch()
    return
  }

  scheduleIDSearchApply()
}

function applyIDSearch() {
  clearIDSearchDebounce()
  replaceRecordListRoute(appliedRecordFilters(), recordState.value.sort)
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
  replaceRecordListRoute(appliedRecordFilters(), recordState.value.sort)
}

function updateFilterValue(id: number, value: string, applyImmediately = false) {
  filterTokens.value = filterTokens.value.map((filter) => (
    filter.id === id ? { ...filter, value } : filter
  ))

  if (applyImmediately) {
    applyFilter(id)
  }
}

function applyFilter(id: number) {
  const filter = filterTokens.value.find((candidate) => candidate.id === id)
  if (!filter) {
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

  replaceRecordListRoute(appliedRecordFilters(), recordState.value.sort)
}

function removeFilter(id: number) {
  filterTokens.value = filterTokens.value.filter((filter) => filter.id !== id)
  replaceRecordListRoute(appliedRecordFilters(), recordState.value.sort)
}

function selectColumnMenuItem(key: string) {
  if (key === 'show-all') {
    hiddenColumnKeys.value = []
    writeHiddenColumnKeys(props.entity, hiddenColumnKeys.value)
  }
}

function updateColumnVisibility(key: string, visible: boolean) {
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
  writeHiddenColumnKeys(props.entity, hiddenColumnKeys.value)

  if (!visible && recordState.value.sort?.key === key) {
    updateSort(null)
  }
}

function hiddenColumnStorageKey(entity: string): string {
  return `dygo.studio.records.hiddenColumns.${entity}`
}

function readHiddenColumnKeys(entity: string): string[] {
  if (typeof window === 'undefined') {
    return []
  }

  try {
    const value = JSON.parse(window.localStorage.getItem(hiddenColumnStorageKey(entity)) ?? '[]')
    if (!Array.isArray(value)) {
      return []
    }

    return value.filter((key): key is string => typeof key === 'string' && key !== 'name')
  } catch {
    return []
  }
}

function writeHiddenColumnKeys(entity: string, keys: string[]) {
  if (typeof window === 'undefined') {
    return
  }

  window.localStorage.setItem(hiddenColumnStorageKey(entity), JSON.stringify(keys.filter((key) => key !== 'name')))
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

function filterInputType(filter: ActiveRecordFilter): string {
  const field = filterFieldForToken(filter)
  switch (field?.type) {
    case 'int':
    case 'bigint':
    case 'decimal':
    case 'currency':
      return 'number'
    case 'date':
      return 'date'
    default:
      return 'text'
  }
}

function filterInputPlaceholder(filter: ActiveRecordFilter): string {
  return filterOperatorArity(filter) === 'range' ? 'start..end' : 'value'
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

function scheduleIDSearchApply() {
  clearIDSearchDebounce()
  idSearchDebounce = setTimeout(() => {
    idSearchDebounce = undefined
    replaceRecordListRoute(appliedRecordFilters(), recordState.value.sort)
  }, ID_SEARCH_DEBOUNCE_MS)
}

function clearIDSearchDebounce() {
  if (idSearchDebounce === undefined) {
    return
  }

  clearTimeout(idSearchDebounce)
  idSearchDebounce = undefined
}

function replaceRecordListRoute(filters: RecordListFilter[], sort: DataTableSort | null) {
  // TODO(filters): debounce route replacement once list filters become heavier than the current metadata-backed query.
  const nextQuery = buildRecordListRouteQuery({ filters, sort })
  if (routeQueriesEqual(route.query, nextQuery)) {
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
  const parsed = parseRecordListRouteQuery(route.query)
  return {
    sort: validRouteSort(parsed.sort),
    filters: parsed.filters.filter(isValidRouteFilter),
  }
}

function validRouteSort(sort: DataTableSort | null): DataTableSort | null {
  if (!sort) {
    return null
  }

  return columns.value.some((column) => column.key === sort.key && column.sortable) ? sort : null
}

function isValidRouteFilter(filter: RecordListFilter): boolean {
  if (filter.field === 'name' && filter.operator === 'contains') {
    return typeof filter.value === 'string' && filter.value.trim() !== ''
  }

  const field = filterableFieldByName.value.get(filter.field)
  const operator = field?.filter?.operators?.find((candidate) => candidate.key === filter.operator)
  if (!field || !operator) {
    return false
  }

  if (operator.arity === 'none') {
    return filter.value === undefined || filter.value === ''
  }

  return typeof filter.value === 'string' && filter.value.trim() !== ''
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

function routeQueriesEqual(left: Record<string, unknown>, right: Record<string, string | string[]>): boolean {
  const leftKeys = Object.keys(left).sort()
  const rightKeys = Object.keys(right).sort()
  if (leftKeys.length !== rightKeys.length || leftKeys.some((key, index) => key !== rightKeys[index])) {
    return false
  }

  return leftKeys.every((key) => queryValues(left[key]).join('\u0000') === queryValues(right[key]).join('\u0000'))
}

function queryValues(value: unknown): string[] {
  if (Array.isArray(value)) {
    return value.map((entry) => entry == null ? '' : String(entry)).sort()
  }

  if (value == null) {
    return ['']
  }

  return [String(value)]
}
</script>

<template>
  <section class="record-list-renderer" aria-label="Record list view">
    <PageToolbar v-if="showToolbar">
      <template #left>
        <div class="record-list-renderer__name-search">
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
            <button class="record-list-renderer__filter-segment record-list-renderer__filter-segment--field" type="button">
              {{ filterLabel(filter) }}
            </button>
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
            <select
              v-if="filterHasValue(filter) && filterFieldForToken(filter)?.type === 'boolean'"
              class="record-list-renderer__filter-segment record-list-renderer__filter-segment--value"
              :value="filter.value"
              :aria-label="`${filterLabel(filter)} value`"
              @change="updateFilterValue(filter.id, ($event.target as HTMLSelectElement).value, true)"
            >
              <option value="true">true</option>
              <option value="false">false</option>
            </select>
            <input
              v-else-if="filterHasValue(filter)"
              class="record-list-renderer__filter-segment record-list-renderer__filter-segment--value"
              :value="filter.value"
              :type="filterInputType(filter)"
              :placeholder="filterInputPlaceholder(filter)"
              :aria-label="`${filterLabel(filter)} value`"
              @input="updateFilterValue(filter.id, ($event.target as HTMLInputElement).value)"
              @keydown.enter.prevent="applyFilter(filter.id)"
            />
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

          <DropdownMenu
            label="Add filter"
            trigger-type="icon"
            :items="filterFieldMenuItems"
            @select="selectFilterField"
          >
            <template #trigger>
              <FunnelPlus :size="14" :stroke-width="1.8" aria-hidden="true" />
            </template>
          </DropdownMenu>
        </div>
      </template>

      <template #right>
        <DropdownMenu
          label="Columns"
          trigger-type="icon"
          :items="columnMenuItems"
          @select="selectColumnMenuItem"
          @update:checked="updateColumnVisibility"
        >
          <template #trigger>
            <Settings2 :size="14" :stroke-width="1.8" aria-hidden="true" />
          </template>
        </DropdownMenu>

        <IconButton label="Sort" disabled>
          <ArrowUpDown :size="14" :stroke-width="1.8" aria-hidden="true" />
        </IconButton>

        <IconButton label="Sidebar" disabled>
          <PanelRightOpen :size="14" :stroke-width="1.8" aria-hidden="true" />
        </IconButton>
      </template>
    </PageToolbar>

    <DataTable
      :columns="visibleColumns"
      :rows="recordState.rows"
      :state="tableState"
      :state-title="tableStateTitle"
      :state-message="tableStateMessage"
      :loading="loading"
      :loading-more="recordState.loadingMore"
      :error="error"
      :footer-error="footerError"
      :page-size="recordState.pageSize"
      :page-size-options="pageSizeOptions"
      :total-rows="recordState.total"
      :has-more="hasMore"
      :sort="recordState.sort"
      :selectable="!readOnly"
      :selected-row-keys="recordState.selectedRowKeys"
      :empty-action-label="readOnly ? '' : 'Add first record'"
      row-activatable
      @update:page-size="updatePageSize"
      @update:selected-row-keys="updateSelectedRowKeys"
      @update:sort="updateSort"
      @row-activate="(row) => emit('open-record', row)"
      @load-more="recordsStore.loadMore(props.entity)"
      @empty-action="createRecord"
    />
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
  max-width: min(320px, 52vw);
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
</style>
