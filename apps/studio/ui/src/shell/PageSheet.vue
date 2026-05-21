<script setup lang="ts">
defineProps<{
  labelledBy?: string
  ariaLabel?: string
}>()
</script>

<template>
  <main class="studio-page-sheet" :aria-labelledby="labelledBy" :aria-label="ariaLabel">
    <slot />
  </main>
</template>

<style scoped>
.studio-page-sheet {
  min-height: calc(100vh - var(--studio-shell-header-height) - (var(--studio-shell-gutter) * 2));
  overflow: hidden;
  border: 1px solid var(--studio-border);
  border-radius: var(--studio-radius-sheet);
  background: var(--studio-surface);
  box-shadow: var(--studio-shadow-sheet);
}

@media (max-width: 720px) {
  .studio-page-sheet {
    min-height: calc(100vh - var(--studio-shell-header-height) - 82px);
    border-radius: 16px;
  }
}
</style>
