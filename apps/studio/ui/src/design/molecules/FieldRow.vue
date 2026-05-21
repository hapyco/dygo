<script setup lang="ts">
withDefaults(defineProps<{
  columns?: 1 | 2 | 3
}>(), {
  columns: 2,
})
</script>

<template>
  <div class="d-field-row" :data-columns="columns">
    <slot />
  </div>
</template>

<style scoped>
.d-field-row {
  display: grid;
  gap: 14px;
}

.d-field-row[data-columns='2'] {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.d-field-row[data-columns='3'] {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

@media (max-width: 760px) {
  .d-field-row[data-columns] {
    grid-template-columns: 1fr;
  }
}
</style>
