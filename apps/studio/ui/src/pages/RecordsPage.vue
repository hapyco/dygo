<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ListFilter, Plus } from '@lucide/vue'

import { getEntityMeta, MetadataApiError, type MetadataEntityMeta } from '@/features/metadata/metadata.api'
import PageHeader from '@/shell/PageHeader.vue'
import type { PageHeaderAction } from '@/shell/types'
import RecordListView from '@/features/records/list/RecordListView.vue'
import { RouteName } from '@/router/routes'

const props = defineProps<{
  entity: string
}>()

const router = useRouter()
const entityMeta = ref<MetadataEntityMeta | null>(null)
const entityLoading = ref(false)
const entityError = ref('')

const entityLabel = computed(() => {
  return entityMeta.value?.label || humanizeEntity(props.entity)
})

const pageSummary = computed(() => {
  if (entityLoading.value) {
    return `Loading metadata for ${entityLabel.value}.`
  }

  if (entityMeta.value?.description) {
    return entityMeta.value.description
  }

  if (entityError.value) {
    return entityError.value
  }

  return `Manage ${entityLabel.value} from metadata-backed views. The list view is the first view; table, kanban, and other layouts can plug into this page later.`
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
    entityError.value = ''
    entityLoading.value = true

    try {
      entityMeta.value = await getEntityMeta(entity)
    } catch (error) {
      entityError.value = error instanceof MetadataApiError ? error.message : 'Studio could not load this entity metadata yet.'
    } finally {
      entityLoading.value = false
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
      eyebrow="Records"
      :title="entityLabel"
      :summary="pageSummary"
      :actions="actions"
    />

    <RecordListView :entity-label="entityLabel" :fields="entityMeta?.fields ?? []" />
  </section>
</template>
