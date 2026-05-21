<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRouter } from 'vue-router'

import DataTable from '@/design/organisms/DataTable.vue'
import type { DataTableColumn, DataTableRowKey } from '@/design/types'
import type { MetadataField } from '@/features/metadata/metadata.api'
import { RouteName } from '@/router/routes'
import { useRecordsStore } from '@/stores/records.store'

const props = defineProps<{
  entity: string
  entityLabel: string
  fields?: MetadataField[]
}>()

const router = useRouter()
const recordsStore = useRecordsStore()

const columns = computed<DataTableColumn[]>(() => {
  const seen = new Set<string>()

  return [
    { key: 'name', label: 'Name' },
    ...(props.fields ?? []).map((field) => ({ key: field.name, label: field.label || field.name })),
    { key: 'created-at', label: 'Created At' },
    { key: 'updated-at', label: 'Updated At' },
  ].filter((column) => {
    if (column.key === 'id' || seen.has(column.key)) {
      return false
    }

    seen.add(column.key)
    return true
  })
})

const recordState = computed(() => recordsStore.entityState(props.entity))
const loading = computed(() => recordState.value.status === 'loading')
const error = computed(() => recordState.value.error?.message ?? '')
const hasMore = computed(() => (
  recordState.value.rows.length < recordState.value.total && !recordState.value.error
))

watch(
  () => props.entity,
  (entity) => {
    void recordsStore.loadInitial(entity)
  },
  { immediate: true },
)

function updatePageSize(value: number) {
  void recordsStore.setPageSize(props.entity, value)
}

function updateSelectedRecordKeys(value: DataTableRowKey[]) {
  recordsStore.setSelectedRowKeys(props.entity, value)
}

function createFirstRecord() {
  void router.push({ name: RouteName.RecordNew, params: { entity: props.entity } })
}
</script>

<template>
  <DataTable
    :columns="columns"
    :rows="recordState.rows"
    :loading="loading"
    :loading-more="recordState.loadingMore"
    :error="error"
    :page-size="recordState.pageSize"
    :total-rows="recordState.total"
    :has-more="hasMore"
    selectable
    :selected-row-keys="recordState.selectedRowKeys"
    :empty-title="`No ${entityLabel} records exist.`"
    empty-action-label="Add first record"
    @update:page-size="updatePageSize"
    @update:selected-row-keys="updateSelectedRecordKeys"
    @load-more="recordsStore.loadMore(props.entity)"
    @empty-action="createFirstRecord"
  />
</template>
