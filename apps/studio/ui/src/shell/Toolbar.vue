<script setup lang="ts">
import { computed, useSlots } from 'vue'

const props = withDefaults(defineProps<{
  ariaLabel?: string
}>(), {
  ariaLabel: 'Toolbar',
})

const slots = useSlots()

const hasLeft = computed(() => slotHasContent('left'))
const hasCenter = computed(() => slotHasContent('default'))
const hasRight = computed(() => slotHasContent('right'))

function slotHasContent(name: 'left' | 'default' | 'right'): boolean {
  return (slots[name]?.() ?? []).length > 0
}
</script>

<template>
  <section class="studio-toolbar" role="toolbar" :aria-label="props.ariaLabel">
    <div v-if="hasLeft" class="studio-toolbar__left">
      <slot name="left" />
    </div>

    <div v-if="hasCenter" class="studio-toolbar__center">
      <slot />
    </div>

    <div v-if="hasRight" class="studio-toolbar__right">
      <slot name="right" />
    </div>
  </section>
</template>

<style scoped>
.studio-toolbar {
  display: grid;
  min-width: 0;
  min-height: 40px;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: start;
  gap: 8px 10px;
  padding: 8px 12px;
}

.studio-toolbar__left,
.studio-toolbar__center,
.studio-toolbar__right {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: 8px;
}

.studio-toolbar__left {
  width: 100%;
  flex-wrap: wrap;
  grid-column: 1;
  grid-row: 1;
}

.studio-toolbar__center {
  grid-column: 1 / -1;
  grid-row: 2;
}

.studio-toolbar__right {
  grid-column: 2;
  grid-row: 1;
  justify-self: end;
}

@media (max-width: 720px) {
  .studio-toolbar {
    grid-template-columns: minmax(0, 1fr);
  }

  .studio-toolbar__left,
  .studio-toolbar__center,
  .studio-toolbar__right {
    width: 100%;
    grid-column: 1;
    grid-row: auto;
    justify-self: stretch;
  }
}
</style>
