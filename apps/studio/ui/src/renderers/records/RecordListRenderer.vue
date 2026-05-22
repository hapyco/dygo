<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Columns3 } from '@lucide/vue'

import DataTable from '@/design/organisms/DataTable.vue'
import DropdownMenu from '@/design/primitives/DropdownMenu.vue'
import type { DataTableRowKey, DataTableSort, DropdownMenuItem } from '@/design/types'
import type { MetadataField } from '@/features/metadata/metadata.api'
import PageToolbar from '@/shell/PageToolbar.vue'
import { useRecordsStore } from '@/stores/records.store'
import { buildRecordListColumns } from './columns'

const props = defineProps<{
  entity: string
  entityLabel: string
  fields: MetadataField[]
}>()

const emit = defineEmits<{
  'create-record': []
  'open-record': [row: Record<string, unknown>]
}>()

const recordsStore = useRecordsStore()
const hiddenColumnKeys = ref<string[]>([])

const columns = computed(() => buildRecordListColumns(props.fields))
const hiddenColumnKeySet = computed(() => new Set(hiddenColumnKeys.value.filter((key) => key !== 'name')))
const visibleColumns = computed(() => columns.value.filter((column) => (
  column.key === 'name' || !hiddenColumnKeySet.value.has(column.key)
)))
const recordState = computed(() => recordsStore.entityState(props.entity))
const loading = computed(() => recordState.value.status === 'idle' || recordState.value.status === 'loading')
const error = computed(() => recordState.value.error?.message ?? '')
const footerError = computed(() => recordState.value.loadMoreError?.message ?? '')
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
  (entity) => {
    hiddenColumnKeys.value = readHiddenColumnKeys(entity)
    void recordsStore.loadInitial(entity)
  },
  { immediate: true },
)

function updatePageSize(value: number) {
  void recordsStore.setPageSize(props.entity, value)
}

function updateSelectedRowKeys(value: DataTableRowKey[]) {
  recordsStore.setSelectedRowKeys(props.entity, value)
}

function updateSort(value: DataTableSort | null) {
  void recordsStore.setSort(props.entity, value)
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
</script>

<template>
  <section class="record-list-renderer" aria-label="Record list view">
    <PageToolbar v-if="showToolbar">
      <template #right>
        <DropdownMenu
          label="Columns"
          :items="columnMenuItems"
          @select="selectColumnMenuItem"
          @update:checked="updateColumnVisibility"
        >
          <template #trigger>
            <Columns3 :size="14" :stroke-width="1.8" aria-hidden="true" />
            Columns
          </template>
        </DropdownMenu>
      </template>
    </PageToolbar>

    <DataTable
      :columns="visibleColumns"
      :rows="recordState.rows"
      :loading="loading"
      :loading-more="recordState.loadingMore"
      :error="error"
      :footer-error="footerError"
      :page-size="recordState.pageSize"
      :total-rows="recordState.total"
      :has-more="hasMore"
      :sort="recordState.sort"
      selectable
      :selected-row-keys="recordState.selectedRowKeys"
      :empty-title="`No ${entityLabel} records exist.`"
      empty-action-label="Add first record"
      row-activatable
      @update:page-size="updatePageSize"
      @update:selected-row-keys="updateSelectedRowKeys"
      @update:sort="updateSort"
      @row-activate="(row) => emit('open-record', row)"
      @load-more="recordsStore.loadMore(props.entity)"
      @empty-action="emit('create-record')"
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
</style>
