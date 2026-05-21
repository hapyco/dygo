<script setup lang="ts">
import type { BadgeVariant } from '../types'

withDefaults(defineProps<{
  variant?: BadgeVariant
}>(), {
  variant: 'neutral',
})
</script>

<template>
  <span class="d-badge" :class="`d-badge--${variant}`">
    <slot />
  </span>
</template>

<style scoped>
.d-badge {
  display: inline-flex;
  min-height: 22px;
  align-items: center;
  gap: 5px;
  border: 1px solid transparent;
  border-radius: 999px;
  padding: 0 8px;
  font-size: 12px;
  font-weight: 650;
  letter-spacing: 0;
  line-height: 1;
  white-space: nowrap;
}

.d-badge--neutral {
  border-color: var(--studio-border);
  background: var(--studio-neutral-soft);
  color: var(--studio-text-muted);
}

.d-badge--accent {
  border-color: oklch(0.49 0.118 248 / 0.22);
  background: var(--studio-accent-soft);
  color: var(--studio-accent-strong);
}

.d-badge--success {
  border-color: oklch(0.52 0.105 156 / 0.22);
  background: var(--studio-success-soft);
  color: var(--studio-success);
}

.d-badge--warning {
  border-color: oklch(0.63 0.13 74 / 0.26);
  background: var(--studio-warning-soft);
  color: oklch(0.42 0.098 74);
}

.d-badge--danger {
  border-color: oklch(0.55 0.15 28 / 0.22);
  background: var(--studio-danger-soft);
  color: var(--studio-danger);
}

.d-badge--info {
  border-color: oklch(0.56 0.12 231 / 0.22);
  background: var(--studio-info-soft);
  color: var(--studio-info);
}
</style>
