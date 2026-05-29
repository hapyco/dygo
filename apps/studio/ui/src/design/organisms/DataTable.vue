<script setup lang="ts">
import { computed, useSlots } from 'vue'
import { AlertCircle, ArrowDown, ArrowUp, Inbox, LockKeyhole, LogIn, Plus } from '@lucide/vue'

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
  DataTableState,
  SegmentedControlOption,
  SegmentedControlValue,
} from '@/design/types'

type StateContent = {
  title: string
  message: string
}

const props = withDefaults(defineProps<{
  columns: DataTableColumn[]
  rows: DataTableRow[]
  rowKey?: string
  state?: DataTableState
  loading?: boolean
  loadingMore?: boolean
  error?: string
  stateTitle?: string
  stateMessage?: string
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
  state: undefined,
  loading: false,
  loadingMore: false,
  error: '',
  stateTitle: '',
  stateMessage: '',
  emptyTitle: 'No records exist.',
  emptyMessage: '',
  emptyActionLabel: '',
  pageSizeOptions: () => [],
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
const slots = useSlots()

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
const effectiveState = computed<DataTableState>(() => {
  if (props.state) {
    return props.state
  }

  if (props.loading) {
    return 'loading'
  }

  if (props.error) {
    return 'error'
  }

  if (props.rows.length === 0) {
    return 'empty'
  }

  return 'ready'
})
const stateContent = computed<StateContent>(() => {
  switch (effectiveState.value) {
    case 'loading':
      return {
        title: props.stateTitle || 'Loading records',
        message: props.stateMessage,
      }
    case 'empty':
      return {
        title: props.stateTitle || props.emptyTitle,
        message: props.stateMessage || props.emptyMessage,
      }
    case 'forbidden':
      return {
        title: props.stateTitle || 'Access restricted',
        message: props.stateMessage || 'You do not have permission to view this record list.',
      }
    case 'unauthenticated':
      return {
        title: props.stateTitle || 'Sign in required',
        message: props.stateMessage || 'Sign in to load this record list.',
      }
    case 'error':
      return {
        title: props.stateTitle || 'Records could not load',
        message: props.stateMessage || props.error,
      }
    case 'ready':
    default:
      return {
        title: '',
        message: '',
      }
  }
})
const isEmpty = computed(() => effectiveState.value === 'empty')
const isBlockingState = computed(() => (
  props.rows.length === 0 && ['loading', 'forbidden', 'unauthenticated', 'error'].includes(effectiveState.value)
))
const showFooter = computed(() => props.rows.length > 0 && !isBlockingState.value)
const selectedRowCount = computed(() => props.selectedRowKeys.length)
const showBulkBar = computed(() => props.selectable && selectedRowCount.value > 0 && !isBlockingState.value)
const hasSideRail = computed(() => Boolean(slots['row-side']))
const selectedRowCountText = computed(() => (
  selectedRowCount.value === 1 ? '1 record selected' : `${selectedRowCount.value} records selected`
))
const statePanelRole = computed(() => {
  if (effectiveState.value === 'loading') {
    return 'status'
  }

  if (effectiveState.value === 'empty') {
    return undefined
  }

  return 'alert'
})
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

function cellText(value: unknown, column: DataTableColumn): string {
  if (column.formatValue) {
    return column.formatValue(value)
  }

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

function clearSelection() {
  emit('update:selectedRowKeys', [])
}

function activationCameFromControl(event: Event): boolean {
  const target = event.target
  if (!(target instanceof Element)) {
    return false
  }

  return Boolean(target.closest('a, button, input, select, textarea, [role="button"], [role="checkbox"], [data-row-control]'))
}

function activateRow(row: DataTableRow, index: number, event: MouseEvent | KeyboardEvent) {
  if (!props.rowActivatable || controlsDisabled.value) {
    return
  }

  if (activationCameFromControl(event)) {
    return
  }

  emit('row-activate', row, rowIdentifier(row, index))
}
</script>

<template>
  <section
    class="data-table"
    :class="{ 'data-table--with-side-rail': hasSideRail }"
    aria-label="Records table"
    :aria-busy="effectiveState === 'loading' ? 'true' : undefined"
  >
    <div
      v-if="isEmpty"
      class="data-table__state"
      data-state="empty"
      :role="statePanelRole"
      aria-live="polite"
    >
      <div class="data-table__state-icon" aria-hidden="true">
        <Inbox :size="22" :stroke-width="1.7" />
      </div>
      <p class="data-table__state-title">{{ stateContent.title }}</p>
      <p v-if="stateContent.message" class="data-table__state-message">{{ stateContent.message }}</p>
      <Button
        v-if="emptyActionLabel"
        class="data-table__state-action"
        type="button"
        variant="secondary"
        @click="emit('emptyAction')"
      >
        <Plus :size="14" :stroke-width="1.9" aria-hidden="true" />
        {{ emptyActionLabel }}
      </Button>
    </div>

    <div
      v-else-if="isBlockingState"
      class="data-table__state"
      :data-state="effectiveState"
      :role="statePanelRole"
      :aria-live="effectiveState === 'loading' ? 'polite' : 'assertive'"
    >
      <Spinner
        v-if="effectiveState === 'loading'"
        size="sm"
        :label="stateContent.title"
      />
      <div v-else class="data-table__state-icon" aria-hidden="true">
        <LockKeyhole v-if="effectiveState === 'forbidden'" :size="22" :stroke-width="1.7" />
        <LogIn v-else-if="effectiveState === 'unauthenticated'" :size="22" :stroke-width="1.7" />
        <AlertCircle v-else :size="22" :stroke-width="1.7" />
      </div>
      <p class="data-table__state-title">{{ stateContent.title }}</p>
      <p v-if="stateContent.message" class="data-table__state-message">{{ stateContent.message }}</p>
    </div>

    <div v-else class="data-table__scroller">
      <div class="data-table__x-scroller">
        <table class="data-table__table">
          <thead>
            <tr>
              <th v-if="selectable" class="data-table__select-cell" scope="col">
                <Checkbox
                  :model-value="allVisibleRowsSelected"
                  :disabled="controlsDisabled || rows.length === 0"
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
                </button>
                <span v-else>{{ column.label }}</span>
              </th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="(row, index) in rows"
              :key="rowIdentifier(row, index)"
              :class="{
                'data-table__row--activatable': rowActivatable,
                'data-table__row--selected': selectable && selectedRowKeySet.has(rowIdentifier(row, index)),
              }"
              :aria-selected="selectable ? selectedRowKeySet.has(rowIdentifier(row, index)) : undefined"
              :tabindex="rowActivatable && !controlsDisabled ? 0 : undefined"
              @click="(event) => activateRow(row, index, event)"
              @keydown.enter.prevent="(event) => activateRow(row, index, event)"
              @keydown.space.prevent="(event) => activateRow(row, index, event)"
            >
              <td
                v-if="selectable"
                class="data-table__select-cell"
                data-row-control
                @click.stop
                @keydown.stop
              >
                <Checkbox
                  :model-value="selectedRowKeySet.has(rowIdentifier(row, index))"
                  :disabled="controlsDisabled"
                  :aria-label="`Select record ${index + 1}`"
                  @update:model-value="(selected) => updateRowSelection(rowIdentifier(row, index), selected)"
                />
              </td>
              <td v-for="column in columns" :key="column.key">
                {{ cellText(row[column.key], column) }}
              </td>
            </tr>
          </tbody>
        </table>

      </div>

      <div v-if="hasSideRail" class="data-table__side-rail" aria-label="Activity">
        <div class="data-table__side-rail-header">
          <span class="data-table__visually-hidden">Activity</span>
        </div>
        <div
          v-for="(row, index) in rows"
          :key="rowIdentifier(row, index)"
          class="data-table__side-rail-row"
          :class="{ 'data-table__side-rail-row--selected': selectable && selectedRowKeySet.has(rowIdentifier(row, index)) }"
        >
          <slot
            name="row-side"
            :row="row"
            :index="index"
            :row-key="rowIdentifier(row, index)"
          />
        </div>
      </div>
    </div>

    <div
      v-if="showBulkBar"
      class="data-table__bulk-bar"
      role="status"
      aria-live="polite"
    >
      <span class="data-table__bulk-count">{{ selectedRowCountText }}</span>
      <div class="data-table__bulk-actions">
        <Button
          type="button"
          variant="ghost"
          :disabled="controlsDisabled"
          @click="clearSelection"
        >
          Clear selection
        </Button>
        <Button type="button" variant="secondary" disabled>
          Export
        </Button>
        <Button type="button" variant="secondary" disabled>
          Delete
        </Button>
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
  --data-table-side-rail-width: 116px;

  display: grid;
  min-height: 0;
  min-width: 0;
  grid-template-rows: minmax(0, 1fr) auto auto;
}

.data-table__scroller {
  display: grid;
  min-width: 0;
  min-height: 0;
  overflow-x: hidden;
  overflow-y: auto;
  position: relative;
  scrollbar-color: oklch(0.58 0.018 246 / 0.3) transparent;
  scrollbar-width: thin;
}

.data-table__scroller::-webkit-scrollbar:vertical {
  width: 4px;
}

.data-table__x-scroller {
  grid-area: 1 / 1;
  min-width: 0;
  overflow-x: auto;
  overflow-y: visible;
  scroll-padding-right: var(--data-table-side-rail-width);
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.data-table--with-side-rail .data-table__x-scroller {
  width: calc(100% - var(--data-table-side-rail-width));
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

.data-table thead th,
.data-table tbody td {
  padding: 9px 12px;
  text-align: left;
  vertical-align: middle;
  white-space: nowrap;
}

.data-table__visually-hidden {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  border: 0;
  margin: -1px;
  white-space: nowrap;
}

.data-table thead th {
  border-bottom: 1px solid var(--studio-border);
}

.data-table .data-table__select-cell {
  width: 38px;
  padding-left: 12px;
  padding-right: 6px;
  line-height: 0;
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

.data-table tbody td {
  border-bottom: 0;
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

.data-table__row--selected td {
  background: var(--studio-accent-soft);
  color: var(--studio-text);
}

.data-table__row--selected:hover td {
  background: oklch(0.941 0.034 248);
}

.data-table__side-rail {
  display: grid;
  grid-area: 1 / 1;
  width: var(--data-table-side-rail-width);
  justify-self: end;
  position: sticky;
  right: 0;
  z-index: 2;
  align-self: start;
  box-shadow: -1px 0 0 var(--studio-border);
  pointer-events: none;
}

.data-table__side-rail-header {
  position: sticky;
  top: 0;
  z-index: 1;
  min-height: 35px;
  border-bottom: 1px solid var(--studio-border);
  background: var(--studio-surface);
}

.data-table__side-rail-row {
  display: flex;
  min-height: 35.55px;
  align-items: center;
  justify-content: flex-end;
  background: var(--studio-surface);
  color: var(--studio-text-muted);
  padding: 9px 12px 9px 8px;
}

.data-table__side-rail-row--selected {
  background: var(--studio-accent-soft);
  color: var(--studio-text);
}

.data-table__state {
  display: grid;
  min-height: 260px;
  align-content: start;
  justify-items: center;
  padding: 196px 16px 44px;
  text-align: center;
}

.data-table__state-icon {
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

.data-table__state[data-state='forbidden'] .data-table__state-icon,
.data-table__state[data-state='unauthenticated'] .data-table__state-icon {
  border-color: oklch(0.56 0.12 231 / 0.24);
  background: var(--studio-info-soft);
  color: var(--studio-info);
}

.data-table__state[data-state='error'] .data-table__state-icon {
  border-color: oklch(0.55 0.15 28 / 0.24);
  background: var(--studio-danger-soft);
  color: var(--studio-danger);
}

.data-table__state-title {
  margin: 12px 0 0;
  color: var(--studio-text);
  font-size: 14px;
  font-weight: 600;
  line-height: 1.3;
}

.data-table__state-message {
  max-width: 42ch;
  margin: 6px 0 0;
  color: var(--studio-text-muted);
  font-size: 13px;
  line-height: 1.45;
}

.data-table__state-action {
  margin-top: 14px;
}

.data-table__bulk-bar {
  display: flex;
  min-height: 48px;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  border-top: 1px solid var(--studio-border);
  background: var(--studio-surface);
  padding: 8px 12px;
}

.data-table__bulk-count {
  color: var(--studio-text);
  font-size: 13px;
  font-weight: 700;
  line-height: 1.2;
}

.data-table__bulk-actions {
  display: flex;
  align-items: center;
  gap: 8px;
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
  .data-table__bulk-bar {
    align-items: flex-start;
    flex-direction: column;
  }

  .data-table__bulk-actions {
    width: 100%;
    flex-wrap: wrap;
  }

  .data-table__footer {
    align-items: stretch;
    flex-direction: column;
  }

  .data-table__footer-right {
    justify-content: space-between;
  }
}
</style>
