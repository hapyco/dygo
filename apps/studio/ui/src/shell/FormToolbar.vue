<script setup lang="ts">
import Toolbar from './Toolbar.vue'

const props = withDefaults(defineProps<{
  ariaLabel?: string
}>(), {
  ariaLabel: 'Form toolbar',
})
</script>

<template>
  <Toolbar :aria-label="props.ariaLabel">
    <template #left>
      <slot name="left" />
    </template>

    <slot />

    <template #right>
      <slot name="right" />
    </template>
  </Toolbar>
</template>
