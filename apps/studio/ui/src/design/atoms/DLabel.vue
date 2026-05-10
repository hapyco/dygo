<script setup lang="ts">
defineProps<{
  for?: string
}>()
</script>

<template>
  <label class="d-label" :for="for">
    <slot />
  </label>
</template>

<style scoped>
.d-label {
  color: var(--studio-text);
  font-size: 13px;
  font-weight: 600;
  line-height: 1.2;
}
</style>
