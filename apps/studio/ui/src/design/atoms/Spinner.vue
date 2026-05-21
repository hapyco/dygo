<script setup lang="ts">
import type { ControlSize } from '../types'

withDefaults(defineProps<{
  size?: ControlSize
  label?: string
}>(), {
  size: 'md',
})
</script>

<template>
  <span
    class="d-spinner"
    :data-size="size"
    :role="label ? 'status' : undefined"
    :aria-label="label || undefined"
    :aria-hidden="label ? undefined : 'true'"
  />
</template>

<style scoped>
.d-spinner {
  display: inline-block;
  width: 16px;
  height: 16px;
  flex: 0 0 auto;
  border: 2px solid currentColor;
  border-right-color: transparent;
  border-radius: 999px;
  animation: d-spinner-spin 700ms linear infinite;
}

.d-spinner[data-size='sm'] {
  width: 13px;
  height: 13px;
  border-width: 1.5px;
}

.d-spinner[data-size='xs'] {
  width: 12px;
  height: 12px;
  border-width: 1.5px;
}

@keyframes d-spinner-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
