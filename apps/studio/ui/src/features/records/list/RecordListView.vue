<script setup lang="ts">
import type { MetadataField } from '@/features/metadata/metadata.api'
import RecordTable from './RecordTable.vue'

defineProps<{
  entity: string
  entityLabel: string
  fields?: MetadataField[]
}>()
</script>

<template>
  <section class="record-list-view" aria-label="Record list view">
    <RecordTable :entity="entity" :entity-label="entityLabel" :fields="fields" />
  </section>
</template>

<style scoped>
.record-list-view {
  display: grid;
  min-width: 0;
  min-height: 0;
  margin: 0 calc(var(--studio-page-padding) * -1) calc(var(--studio-page-padding) * -1);
}
</style>
