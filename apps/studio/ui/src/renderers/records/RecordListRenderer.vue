<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ArrowUpDown, FunnelPlus, PanelRightOpen, Settings2, X } from '@lucide/vue'

import { IconButton, Input } from '@/design'
import DataTable from '@/design/organisms/DataTable.vue'
import DropdownMenu from '@/design/primitives/DropdownMenu.vue'
import type { DataTableRowKey, DataTableSort, DataTableState, DropdownMenuItem } from '@/design/types'
import type { MetadataField } from '@/features/metadata/metadata.api'
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
const hiddenColumnKeys = ref<string[]>([])
// TODO: Wire this to a real name-search query once Record list filtering supports search-style matching.
const nameSearch = ref('')
const dummyFilters = ref<{ id: number; field: string }[]>([])
let nextDummyFilterId = 1

const columns = computed(() => buildRecordListColumns(props.fields, props.systemFields ?? []))
const pageSizeOptions = computed(() => platformStore.recordListPolicy?.['page-sizes'] ?? [])
const filterableFields = computed(() => (
  [...props.fields, ...(props.systemFields ?? [])].filter(isFilterableField)
))
const filterFieldMenuItems = computed<DropdownMenuItem[]>(() => {
  if (filterableFields.value.length === 0) {
    return [{ type: 'item', key: 'empty', label: 'No filterable fields', disabled: true }]
  }

  return [
    { type: 'label', key: 'filter-fields-label', label: 'Fields' },
    ...filterableFields.value.map((field) => ({
      type: 'item' as const,
      key: field.name,
      label: field.label || field.name,
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
const showToolbar = computed(() => recordState.value.rows.length > 0)
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
  () => props.entity,
  async (entity) => {
    hiddenColumnKeys.value = readHiddenColumnKeys(entity)
    await recordsStore.loadInitial(entity)
  },
  { immediate: true },
)

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
  void recordsStore.setSort(props.entity, value)
}

function selectFilterField(key: string) {
  if (!filterableFields.value.some((field) => field.name === key)) {
    return
  }

  dummyFilters.value = [
    ...dummyFilters.value,
    { id: nextDummyFilterId, field: key },
  ]
  nextDummyFilterId += 1
}

function removeDummyFilter(id: number) {
  dummyFilters.value = dummyFilters.value.filter((filter) => filter.id !== id)
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

  return !['collection', 'json', 'password'].includes(field.type)
}

function dummyValueForFilterField(field: MetadataField): string {
  switch (field.type) {
    case 'boolean':
      return 'true'
    case 'datetime':
      return 'today'
    case 'link':
      return 'core.record'
    default:
      return field.name === 'type' ? 'text' : 'value'
  }
}

function dummyFilterFieldMeta(fieldName: string): MetadataField | null {
  return filterableFields.value.find((field) => field.name === fieldName) ?? null
}

function dummyFilterLabel(fieldName: string): string {
  const field = dummyFilterFieldMeta(fieldName)
  return field?.label || field?.name || 'Field'
}

function dummyFilterValue(fieldName: string): string {
  const field = dummyFilterFieldMeta(fieldName)
  return field ? dummyValueForFilterField(field) : 'value'
}
</script>

<template>
  <section class="record-list-renderer" aria-label="Record list view">
    <PageToolbar v-if="showToolbar">
      <template #left>
        <div class="record-list-renderer__name-search">
          <Input
            :model-value="nameSearch"
            type="search"
            placeholder="ID"
            aria-label="Filter records by ID"
            @update:model-value="nameSearch = $event"
          />
        </div>

        <div class="record-list-renderer__filter-controls">
          <div
            v-for="filter in dummyFilters"
            :key="filter.id"
            class="record-list-renderer__filter-token"
            aria-label="Dummy active filter"
          >
            <button class="record-list-renderer__filter-segment record-list-renderer__filter-segment--field" type="button">
              {{ dummyFilterLabel(filter.field) }}
            </button>
            <button class="record-list-renderer__filter-segment record-list-renderer__filter-segment--operator" type="button">
              is
            </button>
            <button class="record-list-renderer__filter-segment record-list-renderer__filter-segment--value" type="button">
              {{ dummyFilterValue(filter.field) }}
            </button>
            <button
              class="record-list-renderer__filter-remove"
              type="button"
              aria-label="Remove dummy filter"
              @click="removeDummyFilter(filter.id)"
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
.record-list-renderer__filter-remove {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  border: 0;
  border-right: 1px solid var(--studio-border);
  background: transparent;
  color: var(--studio-text-muted);
  font: inherit;
  font-size: 13px;
  line-height: 1;
}

.record-list-renderer__filter-segment {
  max-width: 120px;
  padding: 0 8px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.record-list-renderer__filter-segment--field {
  color: var(--studio-text);
  font-weight: 500;
}

.record-list-renderer__filter-segment--operator {
  color: var(--studio-text-subtle);
}

.record-list-renderer__filter-segment--value {
  background: var(--studio-surface);
  color: var(--studio-text);
}

.record-list-renderer__filter-remove {
  width: var(--studio-control-height-xs);
  flex: 0 0 auto;
  justify-content: center;
  border-right: 0;
}

.record-list-renderer__filter-segment:hover,
.record-list-renderer__filter-remove:hover {
  background: var(--studio-surface-raised);
  color: var(--studio-text);
}

.record-list-renderer__filter-segment:focus-visible,
.record-list-renderer__filter-remove:focus-visible {
  outline: 2px solid var(--studio-focus);
  outline-offset: -2px;
}
</style>
