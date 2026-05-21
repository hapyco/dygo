<script setup lang="ts">
defineProps<{
  entity: string
}>()
</script>

<template>
  <section class="studio-page" aria-labelledby="entity-list-title">
    <p class="studio-page__eyebrow">Entity list</p>
    <h1 id="entity-list-title">{{ entity }}</h1>
    <p class="studio-page__summary">
      This route is reserved for the metadata-powered list renderer for {{ entity }} Records.
    </p>
  </section>
</template>
