<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import DataTable from '@/design/organisms/DataTable.vue'
import type { DataTableColumn } from '@/design/types'
import type { MetadataField } from '@/features/metadata/metadata.api'
import { listRecords, RecordApiError, type RecordData } from '@/features/records/records.api'

const props = defineProps<{
  entity: string
  entityLabel: string
  fields?: MetadataField[]
}>()

const pageSize = ref(20)
const records = ref<RecordData[]>([])
const selectedRecordKeys = ref<Array<string | number>>([])
const loading = ref(false)
const loadingMore = ref(false)
const error = ref('')
const lastPageCount = ref(0)

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

const hasMore = computed(() => lastPageCount.value === pageSize.value && !error.value)

watch(
  () => props.entity,
  () => {
    void loadInitialRecords()
  },
  { immediate: true },
)

async function loadInitialRecords() {
  loading.value = true
  error.value = ''
  records.value = []
  selectedRecordKeys.value = []
  lastPageCount.value = 0

  try {
    const result = await listRecords(props.entity, { limit: pageSize.value, offset: 0 })
    records.value = result.data
    lastPageCount.value = result.meta.count
  } catch (caught) {
    error.value = caught instanceof RecordApiError ? caught.message : 'Studio could not load records.'
  } finally {
    loading.value = false
  }
}

async function loadMoreRecords() {
  if (loading.value || loadingMore.value || !hasMore.value) {
    return
  }

  loadingMore.value = true
  error.value = ''

  try {
    const result = await listRecords(props.entity, { limit: pageSize.value, offset: records.value.length })
    records.value = [...records.value, ...result.data]
    lastPageCount.value = result.meta.count
  } catch (caught) {
    error.value = caught instanceof RecordApiError ? caught.message : 'Studio could not load more records.'
  } finally {
    loadingMore.value = false
  }
}

function updatePageSize(value: number) {
  pageSize.value = value
  void loadInitialRecords()
}

function updateSelectedRecordKeys(value: Array<string | number>) {
  selectedRecordKeys.value = value
}
</script>

<template>
  <DataTable
    :columns="columns"
    :rows="records"
    :loading="loading"
    :loading-more="loadingMore"
    :error="error"
    :page-size="pageSize"
    :has-more="hasMore"
    selectable
    :selected-row-keys="selectedRecordKeys"
    :empty-message="`No ${entityLabel} records found.`"
    @update:page-size="updatePageSize"
    @update:selected-row-keys="updateSelectedRecordKeys"
    @load-more="loadMoreRecords"
  />
</template>
