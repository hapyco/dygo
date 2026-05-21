<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  id?: string
  title: string
  description?: string
}>()

const titleId = computed(() => props.id ?? `${props.title.replace(/[^a-zA-Z0-9_-]+/g, '-').toLowerCase()}-title`)
</script>

<template>
  <section class="d-form-section" :aria-labelledby="titleId">
    <header class="d-form-section__header">
      <h2 class="d-form-section__title" :id="titleId">
        {{ title }}
      </h2>
      <p v-if="description" class="d-form-section__description">{{ description }}</p>
    </header>
    <div class="d-form-section__body">
      <slot />
    </div>
  </section>
</template>

<style scoped>
.d-form-section {
  display: grid;
  gap: 16px;
}

.d-form-section__header {
  display: grid;
  gap: 5px;
}

.d-form-section__title {
  margin: 0;
  color: var(--studio-text);
  font-size: 16px;
  font-weight: 700;
  letter-spacing: 0;
  line-height: 1.25;
}

.d-form-section__description {
  max-width: 64ch;
  margin: 0;
  color: var(--studio-text-muted);
  font-size: 13px;
  line-height: 1.45;
}

.d-form-section__body {
  display: grid;
  gap: 14px;
}
</style>
