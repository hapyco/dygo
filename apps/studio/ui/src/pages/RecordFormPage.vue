<script setup lang="ts">
const props = defineProps<{
  entity: string
  id?: string
  mode: 'new' | 'detail'
}>()
</script>

<template>
  <section class="studio-page" aria-labelledby="record-form-title">
    <p class="studio-page__eyebrow">{{ props.mode === 'new' ? 'New Record' : 'Record' }}</p>
    <h1 id="record-form-title">
      <template v-if="props.mode === 'new'">New {{ props.entity }}</template>
      <template v-else>{{ props.entity }} {{ props.id }}</template>
    </h1>
    <p class="studio-page__summary">
      This route is reserved for the metadata-powered Record form. Activity will appear inside this Record page.
    </p>
  </section>
</template>
