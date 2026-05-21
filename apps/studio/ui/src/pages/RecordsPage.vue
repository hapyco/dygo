<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ListFilter, Plus } from '@lucide/vue'

import { getEntityMeta, type MetadataEntityMeta } from '@/features/metadata/metadata.api'
import PageHeader from '@/shell/PageHeader.vue'
import type { PageHeaderAction } from '@/shell/types'
import RecordListView from '@/features/records/list/RecordListView.vue'
import { RouteName } from '@/router/routes'

const props = defineProps<{
  entity: string
}>()

const router = useRouter()
const entityMeta = ref<MetadataEntityMeta | null>(null)

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
    disabled: !entityMeta.value,
    onSelect: () => {
      void router.push({ name: RouteName.RecordNew, params: { entity: props.entity } })
    },
  },
])

watch(
  () => props.entity,
  async (entity) => {
    entityMeta.value = null

    try {
      entityMeta.value = await getEntityMeta(entity)
    } catch {
      entityMeta.value = null
    }
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
