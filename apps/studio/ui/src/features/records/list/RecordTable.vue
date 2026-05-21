<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'

import DataTable from '@/design/organisms/DataTable.vue'
import type { DataTableColumn } from '@/design/types'
import type { MetadataField } from '@/features/metadata/metadata.api'
import { listRecords, RecordApiError, type RecordData } from '@/features/records/records.api'
import { RouteName } from '@/router/routes'

const props = defineProps<{
  entity: string
  entityLabel: string
  fields?: MetadataField[]
}>()

const router = useRouter()
const pageSize = ref(20)
const records = ref<RecordData[]>([])
const selectedRecordKeys = ref<Array<string | number>>([])
const loading = ref(false)
const loadingMore = ref(false)
const error = ref('')
const totalRecords = ref(0)

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

const hasMore = computed(() => records.value.length < totalRecords.value && !error.value)

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
  totalRecords.value = 0

  try {
    const result = await listRecords(props.entity, { limit: pageSize.value, offset: 0 })
    records.value = result.data
    totalRecords.value = result.meta.total ?? result.data.length
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
    totalRecords.value = result.meta.total ?? records.value.length
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

function createFirstRecord() {
  void router.push({ name: RouteName.RecordNew, params: { entity: props.entity } })
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
    :total-rows="totalRecords"
    :has-more="hasMore"
    selectable
    :selected-row-keys="selectedRecordKeys"
    :empty-title="`No ${entityLabel} records exist.`"
    empty-action-label="Add first record"
    @update:page-size="updatePageSize"
    @update:selected-row-keys="updateSelectedRecordKeys"
    @load-more="loadMoreRecords"
    @empty-action="createFirstRecord"
  />
</template>
