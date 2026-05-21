<script setup lang="ts">
import type { ControlSize } from '../types'

withDefaults(defineProps<{
  label: string
  type?: 'button' | 'submit' | 'reset'
  variant?: 'secondary' | 'ghost' | 'danger'
  size?: ControlSize
  loading?: boolean
  disabled?: boolean
}>(), {
  type: 'button',
  variant: 'secondary',
  size: 'md',
  loading: false,
  disabled: false,
})
</script>

<template>
  <button
    class="d-icon-button"
    :class="`d-icon-button--${variant}`"
    :type="type"
    :data-size="size"
    :disabled="disabled || loading"
    :aria-label="label"
    :aria-busy="loading ? 'true' : undefined"
  >
    <span v-if="loading" class="d-icon-button__spinner" aria-hidden="true" />
    <slot v-else />
  </button>
</template>

<style scoped>
.d-icon-button {
  display: inline-flex;
  width: var(--studio-control-height-md);
  height: var(--studio-control-height-md);
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  border: 1px solid transparent;
  border-radius: var(--studio-radius-control);
  color: var(--studio-text-muted);
  transition:
    background-color 160ms ease,
    border-color 160ms ease,
    color 160ms ease,
    box-shadow 160ms ease;
}

.d-icon-button[data-size='sm'] {
  width: var(--studio-control-height-sm);
  height: var(--studio-control-height-sm);
}

.d-icon-button:focus-visible {
  outline: 2px solid var(--studio-focus);
  outline-offset: 2px;
}

.d-icon-button:disabled {
  opacity: 0.58;
}

.d-icon-button--secondary {
  border-color: var(--studio-border);
  background: var(--studio-control-bg);
  box-shadow: var(--studio-shadow-control);
}

.d-icon-button--secondary:hover:not(:disabled) {
  border-color: var(--studio-border-strong);
  background: var(--studio-control-bg-hover);
  color: var(--studio-text);
}

.d-icon-button--ghost {
  background: transparent;
}

.d-icon-button--ghost:hover:not(:disabled) {
  background: var(--studio-surface-raised);
  color: var(--studio-text);
}

.d-icon-button--danger {
  border-color: oklch(0.55 0.15 28 / 0.24);
  background: var(--studio-danger-soft);
  color: var(--studio-danger);
}

.d-icon-button--danger:hover:not(:disabled) {
  border-color: var(--studio-danger);
}

.d-icon-button__spinner {
  width: 14px;
  height: 14px;
  border: 2px solid currentColor;
  border-right-color: transparent;
  border-radius: 999px;
  animation: d-icon-button-spin 700ms linear infinite;
}

@keyframes d-icon-button-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
