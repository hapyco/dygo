<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRoute } from 'vue-router'

import { ErrorState, Spinner } from '@/design'
import { storeError } from '@/stores/status'
import { useNavigationStore } from '@/stores/navigation.store'
import { pageRenderer } from './page-renderers'
import { usePageQuery } from './pages.query'

const props = defineProps<{
  app: string
  pageKey: string
}>()

const pageQuery = usePageQuery(() => props.app, () => props.pageKey)
const page = computed(() => pageQuery.data.value ?? null)
const loading = computed(() => pageQuery.isPending.value)
const renderer = computed(() => page.value ? pageRenderer(page.value.renderer) : null)
const error = computed(() => pageQuery.error.value
  ? storeError(pageQuery.error.value, 'Studio could not load this Page.')
  : null)
const route = useRoute()
const navigation = useNavigationStore()
const sheetPath = route.path
watch(() => page.value?.label, label => {
  if (label) navigation.setTabLabel(sheetPath, label)
})
</script>

<template>
  <section v-if="loading" class="studio-page page-host__state" aria-live="polite">
    <Spinner size="sm" label="Loading Page" />
    <p>Loading Page</p>
  </section>

  <section v-else-if="error" class="studio-page page-host__state">
    <ErrorState title="Page unavailable" :message="error.message" />
  </section>

  <section v-else-if="page && !renderer" class="studio-page page-host__state">
    <ErrorState
      title="Page renderer unavailable"
      :message="`Studio does not provide the ${page.renderer} renderer.`"
    />
  </section>

  <component :is="renderer" v-else-if="page && renderer" :page="page" />
</template>

<style scoped>
.page-host__state {
  justify-items: start;
  padding-top: 196px;
}

.page-host__state p {
  margin: 0;
  color: var(--studio-text-muted);
  font-size: 13px;
  font-weight: 500;
}
</style>
