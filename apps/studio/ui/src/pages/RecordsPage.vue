<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ListFilter, Plus } from '@lucide/vue'

import PageHeader from '@/shell/PageHeader.vue'
import type { PageHeaderAction } from '@/shell/types'
import RecordListView from '@/features/records/list/RecordListView.vue'
import { RouteName } from '@/router/routes'
import { useMetadataStore } from '@/stores/metadata.store'

const props = defineProps<{
  entity: string
}>()

const router = useRouter()
const metadataStore = useMetadataStore()

const entityMeta = computed(() => metadataStore.entityMeta(props.entity))
const entityMetaStatus = computed(() => metadataStore.entityMetaStatus(props.entity))

const entityLabel = computed(() => {
  return entityMeta.value?.label || humanizeEntity(props.entity)
})

const actions = computed<PageHeaderAction[]>(() => [
  {
    label: 'Filter',
    icon: ListFilter,
    variant: 'secondary',
    disabled: true,
  },
  {
    label: 'New record',
    icon: Plus,
    variant: 'primary',
    disabled: entityMetaStatus.value !== 'ready',
    onSelect: () => {
      void router.push({ name: RouteName.RecordNew, params: { entity: props.entity } })
    },
  },
])

watch(
  () => props.entity,
  async (entity) => {
    await metadataStore.loadEntityMeta(entity)
  },
  { immediate: true },
)

function humanizeEntity(value: string): string {
  return value
    .replace(/[-_]+/g, ' ')
    .replace(/\b\w/g, (letter) => letter.toUpperCase())
}
</script>

<template>
  <section class="studio-page records-page" aria-labelledby="records-page-title">
    <PageHeader
      title-id="records-page-title"
      :title="entityLabel"
      :actions="actions"
    />

    <RecordListView :entity="props.entity" :entity-label="entityLabel" :fields="entityMeta?.fields ?? []" />
  </section>
</template>

<style scoped>
.records-page {
  gap: 0;
  grid-template-rows: auto minmax(0, 1fr);
  height: 100%;
  min-height: 0;
}
</style>
