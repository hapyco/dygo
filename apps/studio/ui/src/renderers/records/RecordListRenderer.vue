<script setup lang="ts">
import { computed, watch } from 'vue'

import DataTable from '@/design/organisms/DataTable.vue'
import type { DataTableRowKey } from '@/design/types'
import type { MetadataField } from '@/features/metadata/metadata.api'
import { useRecordsStore } from '@/stores/records.store'
import { buildRecordListColumns } from './columns'

const props = defineProps<{
  entity: string
  entityLabel: string
  fields: MetadataField[]
}>()

const emit = defineEmits<{
  'create-record': []
}>()

const recordsStore = useRecordsStore()

const columns = computed(() => buildRecordListColumns(props.fields))
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

function updateSelectedRowKeys(value: DataTableRowKey[]) {
  recordsStore.setSelectedRowKeys(props.entity, value)
}
</script>

<template>
  <section class="record-list-renderer" aria-label="Record list view">
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
      @update:selected-row-keys="updateSelectedRowKeys"
      @load-more="recordsStore.loadMore(props.entity)"
      @empty-action="emit('create-record')"
    />
  </section>
</template>

<style scoped>
.record-list-renderer {
  display: grid;
  min-width: 0;
  min-height: 0;
  margin: 0 calc(var(--studio-page-padding) * -1) calc(var(--studio-page-padding) * -1);
}
</style>
