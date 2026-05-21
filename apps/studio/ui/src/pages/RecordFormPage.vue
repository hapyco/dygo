<script setup lang="ts">
import { computed } from 'vue'

import PageHeader from '@/shell/PageHeader.vue'

const props = defineProps<{
  entity: string
  id?: string
  mode: 'new' | 'detail'
}>()

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
      :title="pageTitle"
    />
  </section>
</template>
