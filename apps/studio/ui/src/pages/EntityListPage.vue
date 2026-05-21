<script setup lang="ts">
import PageHeader from '@/shell/PageHeader.vue'

defineProps<{
  entity: string
}>()
</script>

<template>
  <section class="studio-page" aria-labelledby="entity-list-title">
    <PageHeader
      title-id="entity-list-title"
      eyebrow="Entity list"
      :title="entity"
      :summary="`This route is reserved for the metadata-powered list renderer for ${entity} Records.`"
    />
  </section>
</template>
