<script setup lang="ts">
defineProps<{
  title?: string
  message: string
}>()
</script>

<template>
  <div class="d-error-state" role="alert">
    <strong v-if="title" class="d-error-state__title">{{ title }}</strong>
    <p class="d-error-state__message">{{ message }}</p>
  </div>
</template>

<style scoped>
.d-error-state {
  display: grid;
  gap: 3px;
  border: 1px solid oklch(0.55 0.15 28 / 0.24);
  border-radius: 7px;
  background: oklch(0.98 0.02 28);
  color: var(--studio-danger);
  padding: 10px 12px;
}

.d-error-state__title {
  font-size: 13px;
  line-height: 1.3;
}

.d-error-state__message {
  margin: 0;
  font-size: 13px;
  line-height: 1.45;
}
</style>
