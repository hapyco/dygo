<script setup lang="ts">
import { computed, useSlots } from 'vue'

const props = withDefaults(defineProps<{
  ariaLabel?: string
}>(), {
  ariaLabel: 'Page toolbar',
})

const slots = useSlots()

const hasLeft = computed(() => Boolean(slots.left))
const hasCenter = computed(() => Boolean(slots.default))
const hasRight = computed(() => Boolean(slots.right))
</script>

<template>
  <section class="studio-page-toolbar" role="toolbar" :aria-label="props.ariaLabel">
    <div v-if="hasLeft" class="studio-page-toolbar__left">
      <slot name="left" />
    </div>

    <div v-if="hasCenter" class="studio-page-toolbar__center">
      <slot />
    </div>

    <div v-if="hasRight" class="studio-page-toolbar__right">
      <slot name="right" />
    </div>
  </section>
</template>

<style scoped>
.studio-page-toolbar {
  display: grid;
  min-width: 0;
  min-height: 40px;
  grid-template-columns: minmax(0, 1fr) auto auto;
  align-items: center;
  gap: 10px;
  border-bottom: 1px solid var(--studio-border);
  padding: 8px 12px;
}

.studio-page-toolbar__left,
.studio-page-toolbar__center,
.studio-page-toolbar__right {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: 8px;
}

.studio-page-toolbar__left {
  justify-self: start;
}

.studio-page-toolbar__center {
  justify-self: start;
}

.studio-page-toolbar__right {
  justify-self: end;
}

@media (max-width: 720px) {
  .studio-page-toolbar {
    grid-template-columns: minmax(0, 1fr);
  }

  .studio-page-toolbar__left,
  .studio-page-toolbar__center,
  .studio-page-toolbar__right {
    justify-self: stretch;
  }
}
</style>
