<script setup lang="ts">
import { computed } from 'vue'
import { AlertCircle, ArrowDown, ArrowUp, ArrowUpDown, Inbox, Plus } from '@lucide/vue'

import Button from '@/design/atoms/Button.vue'
import Checkbox from '@/design/atoms/Checkbox.vue'
import Spinner from '@/design/atoms/Spinner.vue'
import SegmentedControl from '@/design/molecules/SegmentedControl.vue'
import type {
  DataTableColumn,
  DataTableRow,
  DataTableRowKey,
  DataTableSort,
  DataTableSortDirection,
  SegmentedControlOption,
  SegmentedControlValue,
} from '@/design/types'

const props = withDefaults(defineProps<{
  columns: DataTableColumn[]
  rows: DataTableRow[]
  rowKey?: string
  loading?: boolean
  loadingMore?: boolean
  error?: string
  emptyTitle?: string
  emptyMessage?: string
  emptyActionLabel?: string
  pageSize: number
  totalRows?: number
  pageSizeOptions?: number[]
  hasMore?: boolean
  sort?: DataTableSort | null
  footerError?: string
  selectable?: boolean
  selectedRowKeys?: DataTableRowKey[]
  rowActivatable?: boolean
}>(), {
  rowKey: 'id',
  loading: false,
  loadingMore: false,
  error: '',
  emptyTitle: 'No records exist.',
  emptyMessage: '',
  emptyActionLabel: '',
  pageSizeOptions: () => [20, 100, 500, 2500],
  hasMore: false,
  sort: null,
  footerError: '',
  selectable: false,
  selectedRowKeys: () => [],
  rowActivatable: false,
})

const emit = defineEmits<{
  'update:pageSize': [value: number]
  'update:selectedRowKeys': [value: DataTableRowKey[]]
  'update:sort': [value: DataTableSort | null]
  'row-activate': [row: DataTableRow, key: DataTableRowKey]
  loadMore: []
  emptyAction: []
}>()

const pageSizeControlOptions = computed<SegmentedControlOption[]>(() => (
  props.pageSizeOptions.map((option) => ({
    value: option,
    label: String(option),
  }))
))
const selectedRowKeySet = computed(() => new Set(props.selectedRowKeys))
const visibleRowKeys = computed(() => props.rows.map((row, index) => rowIdentifier(row, index)))
const allVisibleRowsSelected = computed(() => (
  visibleRowKeys.value.length > 0 && visibleRowKeys.value.every((key) => selectedRowKeySet.value.has(key))
))
const isEmpty = computed(() => !props.loading && !props.error && props.rows.length === 0)
const isBlockingState = computed(() => props.rows.length === 0 && (props.loading || Boolean(props.error)))
const showFooter = computed(() => !isEmpty.value && !isBlockingState.value)
const footerCountText = computed(() => (
  typeof props.totalRows === 'number' ? `${props.rows.length} of ${props.totalRows}` : `${props.rows.length} loaded`
))
const controlsDisabled = computed(() => props.loading || props.loadingMore)

function rowIdentifier(row: DataTableRow, index: number): DataTableRowKey {
  const value = row[props.rowKey]
  if (typeof value === 'string' || typeof value === 'number') {
    return value
  }

  return index
}

function cellText(value: unknown): string {
  if (value === null || value === undefined || value === '') {
    return '-'
  }

  if (typeof value === 'string') {
    return value
  }

  if (typeof value === 'number' || typeof value === 'bigint' || typeof value === 'boolean') {
    return String(value)
  }

  try {
    return JSON.stringify(value)
  } catch {
    return String(value)
  }
}

function updatePageSize(value: SegmentedControlValue) {
  const nextValue = Number(value)
  if (Number.isFinite(nextValue)) {
    emit('update:pageSize', nextValue)
  }
}

function sortDirectionForColumn(column: DataTableColumn): DataTableSortDirection | null {
  if (props.sort?.key !== column.key) {
    return null
  }

  return props.sort.direction
}

function ariaSortForColumn(column: DataTableColumn): 'ascending' | 'descending' | 'none' | undefined {
  if (!column.sortable) {
    return undefined
  }

  const direction = sortDirectionForColumn(column)
  if (direction === 'asc') {
    return 'ascending'
  }

  if (direction === 'desc') {
    return 'descending'
  }

  return 'none'
}

function updateSort(column: DataTableColumn) {
  if (!column.sortable || controlsDisabled.value) {
    return
  }

  const currentDirection = sortDirectionForColumn(column)
  if (!currentDirection) {
    emit('update:sort', { key: column.key, direction: 'asc' })
    return
  }

  if (currentDirection === 'asc') {
    emit('update:sort', { key: column.key, direction: 'desc' })
    return
  }

  emit('update:sort', null)
}

function updateRowSelection(key: DataTableRowKey, selected: boolean) {
  const nextKeys = new Set(props.selectedRowKeys)

  if (selected) {
    nextKeys.add(key)
  } else {
    nextKeys.delete(key)
  }

  emit('update:selectedRowKeys', Array.from(nextKeys))
}

function updateVisibleRowSelection(selected: boolean) {
  const nextKeys = new Set(props.selectedRowKeys)

  visibleRowKeys.value.forEach((key) => {
    if (selected) {
      nextKeys.add(key)
    } else {
      nextKeys.delete(key)
    }
  })

  emit('update:selectedRowKeys', Array.from(nextKeys))
}

function activateRow(row: DataTableRow, index: number) {
  if (!props.rowActivatable || controlsDisabled.value) {
    return
  }

  emit('row-activate', row, rowIdentifier(row, index))
}
</script>

<template>
  <section class="data-table" aria-label="Records table">
    <div v-if="isEmpty" class="data-table__empty">
      <div class="data-table__empty-icon" aria-hidden="true">
        <Inbox :size="22" :stroke-width="1.7" />
      </div>
      <p class="data-table__empty-title">{{ emptyTitle }}</p>
      <p v-if="emptyMessage" class="data-table__empty-message">{{ emptyMessage }}</p>
      <Button
        v-if="emptyActionLabel"
        class="data-table__empty-action"
        type="button"
        variant="secondary"
        @click="emit('emptyAction')"
      >
        <Plus :size="14" :stroke-width="1.9" aria-hidden="true" />
        {{ emptyActionLabel }}
      </Button>
    </div>

    <div v-else-if="isBlockingState" class="data-table__blocking-state">
      <Spinner
        v-if="loading"
        size="sm"
        label="Loading records"
      />
      <div v-else class="data-table__blocking-icon" aria-hidden="true">
        <AlertCircle :size="22" :stroke-width="1.7" />
      </div>
      <p class="data-table__blocking-title">
        {{ loading ? 'Loading records' : error }}
      </p>
    </div>

    <div v-else class="data-table__scroller">
      <div class="data-table__x-scroller">
        <table class="data-table__table">
          <thead>
            <tr>
              <th v-if="selectable" class="data-table__select-cell" scope="col">
                <Checkbox
                  :model-value="allVisibleRowsSelected"
                  :disabled="loading || rows.length === 0"
                  aria-label="Select all records"
                  @update:model-value="updateVisibleRowSelection"
                />
              </th>
              <th
                v-for="column in columns"
                :key="column.key"
                scope="col"
                :aria-sort="ariaSortForColumn(column)"
              >
                <button
                  v-if="column.sortable"
                  class="data-table__header-button"
                  type="button"
                  :disabled="controlsDisabled"
                  @click="updateSort(column)"
                >
                  <span>{{ column.label }}</span>
                  <ArrowUp v-if="sortDirectionForColumn(column) === 'asc'" :size="13" :stroke-width="1.9" aria-hidden="true" />
                  <ArrowDown v-else-if="sortDirectionForColumn(column) === 'desc'" :size="13" :stroke-width="1.9" aria-hidden="true" />
                  <ArrowUpDown v-else :size="13" :stroke-width="1.9" aria-hidden="true" />
                </button>
                <span v-else>{{ column.label }}</span>
              </th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="(row, index) in rows"
              :key="rowIdentifier(row, index)"
              :class="{ 'data-table__row--activatable': rowActivatable }"
              :tabindex="rowActivatable && !controlsDisabled ? 0 : undefined"
              @click="activateRow(row, index)"
              @keydown.enter.prevent="activateRow(row, index)"
              @keydown.space.prevent="activateRow(row, index)"
            >
              <td v-if="selectable" class="data-table__select-cell" @click.stop @keydown.stop>
                <Checkbox
                  :model-value="selectedRowKeySet.has(rowIdentifier(row, index))"
                  :disabled="controlsDisabled"
                  :aria-label="`Select record ${index + 1}`"
                  @update:model-value="(selected) => updateRowSelection(rowIdentifier(row, index), selected)"
                />
              </td>
              <td v-for="column in columns" :key="column.key">
                {{ cellText(row[column.key]) }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <footer v-if="showFooter" class="data-table__footer">
      <SegmentedControl
        :model-value="pageSize"
        :options="pageSizeControlOptions"
        :disabled="controlsDisabled"
        aria-label="Rows to load"
        @update:model-value="updatePageSize"
      />

      <div class="data-table__footer-right">
        <span v-if="footerError" class="data-table__footer-error">{{ footerError }}</span>
        <span class="data-table__count">{{ footerCountText }}</span>
        <Button
          v-if="hasMore || loadingMore"
          type="button"
          variant="secondary"
          :disabled="controlsDisabled"
          :loading="loadingMore"
          @click="emit('loadMore')"
        >
          Load more
        </Button>
      </div>
    </footer>
  </section>
</template>

<style scoped>
.data-table {
  display: grid;
  min-height: 0;
  min-width: 0;
  grid-template-rows: minmax(0, 1fr) auto;
}

.data-table__scroller {
  min-width: 0;
  min-height: 0;
  overflow-x: hidden;
  overflow-y: auto;
  scrollbar-color: oklch(0.58 0.018 246 / 0.3) transparent;
  scrollbar-width: thin;
}

.data-table__scroller::-webkit-scrollbar:vertical {
  width: 4px;
}

.data-table__x-scroller {
  min-width: 0;
  overflow-x: auto;
  overflow-y: visible;
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.data-table__x-scroller::-webkit-scrollbar {
  display: none;
}

.data-table__table {
  width: 100%;
  min-width: 720px;
  border-collapse: separate;
  border-spacing: 0;
}

.data-table th,
.data-table td {
  border-bottom: 1px solid var(--studio-border);
  padding: 9px 12px;
  text-align: left;
  vertical-align: middle;
  white-space: nowrap;
}

.data-table .data-table__select-cell {
  width: 38px;
  padding-left: 12px;
  padding-right: 6px;
}

.data-table th {
  position: sticky;
  top: 0;
  z-index: 1;
  background: var(--studio-surface);
  color: var(--studio-text-subtle);
  font-size: 12px;
  font-weight: 600;
  line-height: 1.2;
}

.data-table__header-button {
  display: inline-flex;
  max-width: 100%;
  align-items: center;
  gap: 6px;
  border: 0;
  background: transparent;
  color: inherit;
  cursor: pointer;
  font: inherit;
  line-height: inherit;
  padding: 0;
}

.data-table__header-button:hover:not(:disabled) {
  color: var(--studio-text);
}

.data-table__header-button:focus-visible {
  border-radius: 4px;
  outline: 2px solid var(--studio-focus);
  outline-offset: 2px;
}

.data-table__header-button:disabled {
  cursor: default;
}

.data-table td {
  color: var(--studio-text-muted);
  font-size: 13px;
  font-weight: 400;
  line-height: 1.35;
}

.data-table tbody tr:hover td {
  background: var(--studio-surface-raised);
  color: var(--studio-text);
}

.data-table__row--activatable {
  cursor: pointer;
}

.data-table__row--activatable:focus-visible {
  outline: 2px solid var(--studio-focus);
  outline-offset: -2px;
}

.data-table__empty {
  display: grid;
  min-height: 260px;
  align-content: start;
  justify-items: center;
  padding: 196px 16px 44px;
  text-align: center;
}

.data-table__empty-icon {
  display: inline-flex;
  width: 38px;
  height: 38px;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--studio-border);
  border-radius: var(--studio-radius-control);
  background: var(--studio-surface-raised);
  color: var(--studio-text-subtle);
}

.data-table__empty-title {
  margin: 12px 0 0;
  color: var(--studio-text);
  font-size: 14px;
  font-weight: 600;
  line-height: 1.3;
}

.data-table__empty-message {
  max-width: 42ch;
  margin: 6px 0 0;
  color: var(--studio-text-muted);
  font-size: 13px;
  line-height: 1.45;
}

.data-table__empty-action {
  margin-top: 14px;
}

.data-table__blocking-state {
  display: grid;
  min-height: 260px;
  align-content: start;
  justify-items: center;
  padding: 196px 16px 44px;
  text-align: center;
}

.data-table__blocking-icon {
  display: inline-flex;
  width: 38px;
  height: 38px;
  align-items: center;
  justify-content: center;
  border: 1px solid oklch(0.55 0.15 28 / 0.24);
  border-radius: var(--studio-radius-control);
  background: var(--studio-danger-soft);
  color: var(--studio-danger);
}

.data-table__blocking-title {
  max-width: 46ch;
  margin: 12px 0 0;
  color: var(--studio-text-muted);
  font-size: 13px;
  font-weight: 500;
  line-height: 1.45;
}

.data-table__footer {
  display: flex;
  min-height: 48px;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  border-bottom: 1px solid var(--studio-border);
  padding: 8px 12px;
}

.data-table__footer-right {
  display: inline-flex;
  align-items: center;
  gap: 10px;
}

.data-table__footer-error {
  max-width: 34ch;
  overflow: hidden;
  color: var(--studio-danger);
  font-size: 12px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.data-table__count {
  color: var(--studio-text-subtle);
  font-size: 12px;
  font-weight: 600;
}

@media (max-width: 720px) {
  .data-table__footer {
    align-items: stretch;
    flex-direction: column;
  }

  .data-table__footer-right {
    justify-content: space-between;
  }
}
</style>
