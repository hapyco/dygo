<script setup lang="ts">
import { computed } from 'vue'

import Button from '@/design/atoms/Button.vue'
import Checkbox from '@/design/atoms/Checkbox.vue'
import SegmentedControl from '@/design/molecules/SegmentedControl.vue'
import type {
  DataTableColumn,
  DataTableRow,
  DataTableRowKey,
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
  emptyMessage?: string
  pageSize: number
  pageSizeOptions?: number[]
  hasMore?: boolean
  selectable?: boolean
  selectedRowKeys?: DataTableRowKey[]
}>(), {
  rowKey: 'id',
  loading: false,
  loadingMore: false,
  error: '',
  emptyMessage: 'No records found.',
  pageSizeOptions: () => [20, 100, 500, 2500],
  hasMore: false,
  selectable: false,
  selectedRowKeys: () => [],
})

const emit = defineEmits<{
  'update:pageSize': [value: number]
  'update:selectedRowKeys': [value: DataTableRowKey[]]
  loadMore: []
}>()

const columnSpan = computed(() => Math.max(props.columns.length + (props.selectable ? 1 : 0), 1))
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
</script>

<template>
  <section class="data-table" aria-label="Records table">
    <div class="data-table__scroller">
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
              <th v-for="column in columns" :key="column.key" scope="col">
                {{ column.label }}
              </th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading">
              <td :colspan="columnSpan" class="data-table__state">
                Loading records
              </td>
            </tr>
            <tr v-else-if="error">
              <td :colspan="columnSpan" class="data-table__state data-table__state--error">
                {{ error }}
              </td>
            </tr>
            <tr v-else-if="rows.length === 0">
              <td :colspan="columnSpan" class="data-table__state">
                {{ emptyMessage }}
              </td>
            </tr>
            <tr v-for="(row, index) in rows" v-else :key="rowIdentifier(row, index)">
              <td v-if="selectable" class="data-table__select-cell">
                <Checkbox
                  :model-value="selectedRowKeySet.has(rowIdentifier(row, index))"
                  :disabled="loading || loadingMore"
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

    <footer class="data-table__footer">
      <SegmentedControl
        :model-value="pageSize"
        :options="pageSizeControlOptions"
        :disabled="loading || loadingMore"
        aria-label="Rows to load"
        @update:model-value="updatePageSize"
      />

      <div class="data-table__footer-right">
        <span class="data-table__count">{{ rows.length }} loaded</span>
        <Button
          type="button"
          variant="secondary"
          size="sm"
          :disabled="loading || loadingMore || !hasMore"
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

.data-table__state {
  height: 104px;
  color: var(--studio-text-subtle);
  text-align: center;
}

.data-table__state--error {
  color: var(--studio-danger);
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
