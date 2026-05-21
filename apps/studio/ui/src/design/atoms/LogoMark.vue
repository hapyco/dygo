<script setup lang="ts">
withDefaults(defineProps<{
  label?: string
  mark?: string
}>(), {
  label: 'dygo Studio',
  mark: 'd',
})
</script>

<template>
  <span class="d-logo-mark" :aria-label="label">
    <span aria-hidden="true">{{ mark }}</span>
  </span>
</template>

<style scoped>
.d-logo-mark {
  display: inline-flex;
  width: 32px;
  height: 32px;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--studio-border);
  border-radius: 8px;
  background: var(--studio-surface);
  color: var(--studio-text);
  font-size: 16px;
  font-weight: 800;
  letter-spacing: 0;
  line-height: 1;
  text-transform: uppercase;
}
</style>
