<script setup lang="ts">
import { computed } from 'vue'

import PageHeader from '@/shell/PageHeader.vue'

const props = defineProps<{
  entity: string
  id?: string
  mode: 'new' | 'detail'
}>()

const pageEyebrow = computed(() => (props.mode === 'new' ? 'New Record' : 'Record'))
const pageTitle = computed(() => {
  if (props.mode === 'new') {
    return `New ${props.entity}`
  }

  return `${props.entity} ${props.id}`
})
</script>

<template>
  <section class="studio-page" aria-labelledby="record-form-title">
    <PageHeader
      title-id="record-form-title"
      :eyebrow="pageEyebrow"
      :title="pageTitle"
      summary="This route is reserved for the metadata-powered Record form. Activity will appear inside this Record page."
    />
  </section>
</template>
