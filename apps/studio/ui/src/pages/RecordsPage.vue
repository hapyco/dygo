<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { Plus } from '@lucide/vue'

import { ErrorState, Spinner } from '@/design'
import { useMetadataEntityMetaQuery } from '@/features/metadata/metadata.query'
import PageHeader from '@/shell/PageHeader.vue'
import type { PageHeaderAction } from '@/shell/types'
import { RecordListRenderer } from '@/renderers/records'
import { RouteName } from '@/router/routes'
import { statusForError, storeError, type LoadStatus } from '@/stores/status'
import RecordFormPage from './RecordFormPage.vue'

const props = defineProps<{
  entity: string
}>()

const router = useRouter()
const entityMetaQuery = useMetadataEntityMetaQuery(() => props.entity)

const entityMeta = computed(() => entityMetaQuery.data.value ?? null)
const entityMetaError = computed(() => (
  entityMetaQuery.error.value
    ? storeError(entityMetaQuery.error.value, 'Studio could not load entity metadata.')
    : null
))
const entityMetaStatus = computed<LoadStatus>(() => {
  if (entityMetaQuery.isPending.value) {
    return 'loading'
  }

  if (entityMetaError.value) {
    return statusForError(entityMetaError.value)
  }

  return entityMeta.value ? 'ready' : 'idle'
})
const isSingle = computed(() => entityMeta.value?.['is-single'] === true)
const isSystem = computed(() => entityMeta.value?.['is-system'] === true)
const canShowList = computed(() => entityMetaStatus.value === 'ready' && !isSingle.value)

const entityLabel = computed(() => {
  return entityMeta.value?.label || humanizeEntity(props.entity)
})

function openNewRecord() {
  if (isSystem.value) {
    return
  }
  void router.push({ name: RouteName.RecordNew, params: { entity: props.entity } })
}

function openRecord(row: Record<string, unknown>) {
  const recordName = row.name
  if (typeof recordName !== 'string' || recordName.length === 0) {
    return
  }

  void router.push({ name: RouteName.RecordDetail, params: { entity: props.entity, recordName } })
}

const actions = computed<PageHeaderAction[]>(() => {
  const next: PageHeaderAction[] = []
  if (!isSystem.value) {
    next.push({
      label: 'New record',
      icon: Plus,
      variant: 'primary',
      disabled: entityMetaStatus.value !== 'ready',
      onSelect: openNewRecord,
    })
  }

  return next
})

function humanizeEntity(value: string): string {
  return value
    .replace(/[-_]+/g, ' ')
    .replace(/\b\w/g, (letter) => letter.toUpperCase())
}
</script>

<template>
  <RecordFormPage
    v-if="isSingle"
    :entity="props.entity"
    mode="single"
  />

  <section v-else class="studio-page records-page" :aria-label="entityLabel">
    <PageHeader
      :show-title="false"
      :system="isSystem"
      :actions="canShowList ? actions : []"
    />

    <div v-if="entityMetaStatus === 'loading' || entityMetaStatus === 'idle'" class="records-page__state">
      <Spinner size="sm" label="Loading entity" />
      <p>Loading entity</p>
    </div>

    <ErrorState
      v-else-if="entityMetaError"
      title="Entity unavailable"
      :message="entityMetaError.message"
    />

    <RecordListRenderer
      v-else-if="canShowList"
      :entity="props.entity"
      :entity-label="entityLabel"
      :fields="entityMeta?.fields ?? []"
      :system-fields="entityMeta?.['system-fields'] ?? []"
      :read-only="isSystem"
      @create-record="openNewRecord"
      @open-record="openRecord"
    />
  </section>
</template>

<style scoped>
.records-page {
  gap: 0;
  grid-template-rows: auto minmax(0, 1fr);
  height: 100%;
  min-height: 0;
}

.records-page__state {
  display: grid;
  justify-items: start;
  gap: 10px;
  padding: 196px 16px 44px;
}

.records-page__state p {
  margin: 0;
  color: var(--studio-text-muted);
  font-size: 13px;
  font-weight: 500;
}
</style>
