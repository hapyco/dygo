<script setup lang="ts">
withDefaults(defineProps<{
  orientation?: 'horizontal' | 'vertical'
}>(), {
  orientation: 'horizontal',
})
</script>

<template>
  <span class="d-divider" :data-orientation="orientation" role="separator" :aria-orientation="orientation" />
</template>

<style scoped>
.d-divider {
  display: block;
  flex: 0 0 auto;
  background: var(--studio-border);
}

.d-divider[data-orientation='horizontal'] {
  width: 100%;
  height: 1px;
}

.d-divider[data-orientation='vertical'] {
  width: 1px;
  min-height: 1em;
  align-self: stretch;
}
</style>
