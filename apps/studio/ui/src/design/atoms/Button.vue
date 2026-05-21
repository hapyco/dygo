<script setup lang="ts">
import type { ControlSize } from '../types'

withDefaults(
  defineProps<{
    type?: 'button' | 'submit' | 'reset'
    variant?: 'primary' | 'secondary' | 'ghost'
    size?: ControlSize
    loading?: boolean
    disabled?: boolean
  }>(),
  {
    type: 'button',
    variant: 'primary',
    size: 'xs',
    loading: false,
    disabled: false,
  },
)
</script>

<template>
  <button
    class="d-button"
    :class="`d-button--${variant}`"
    :type="type"
    :data-size="size"
    :disabled="disabled || loading"
    :aria-busy="loading ? 'true' : undefined"
  >
    <span v-if="loading" class="d-button__spinner" aria-hidden="true" />
    <slot />
  </button>
</template>

<style scoped>
.d-button {
  display: inline-flex;
  min-height: var(--studio-control-height-xs);
  align-items: center;
  justify-content: center;
  gap: 6px;
  border: 1px solid transparent;
  border-radius: var(--studio-radius-control);
  padding: 0 10px;
  font-size: 13px;
  font-weight: 600;
  line-height: 1;
  transition:
    background-color 180ms ease,
    border-color 180ms ease,
    color 180ms ease,
    box-shadow 180ms ease;
}

.d-button[data-size='sm'] {
  min-height: var(--studio-control-height-sm);
  padding: 0 11px;
}

.d-button[data-size='md'] {
  min-height: var(--studio-control-height-md);
  gap: 8px;
  padding: 0 14px;
  font-size: 14px;
}

.d-button:focus-visible {
  outline: 2px solid var(--studio-focus);
  outline-offset: 2px;
}

.d-button:disabled {
  opacity: 0.58;
}

.d-button--primary {
  background: var(--studio-accent);
  color: oklch(0.99 0.004 246);
}

.d-button--primary:hover:not(:disabled) {
  background: var(--studio-accent-strong);
}

.d-button--secondary {
  border-color: var(--studio-border);
  background: var(--studio-control-bg);
  box-shadow: var(--studio-shadow-control);
  color: var(--studio-text);
}

.d-button--secondary:hover:not(:disabled) {
  border-color: var(--studio-border-strong);
  background: var(--studio-control-bg-hover);
}

.d-button--ghost {
  background: transparent;
  color: var(--studio-text-muted);
}

.d-button--ghost:hover:not(:disabled) {
  background: var(--studio-surface-raised);
  color: var(--studio-text);
}

.d-button__spinner {
  width: 14px;
  height: 14px;
  border: 2px solid currentColor;
  border-right-color: transparent;
  border-radius: 999px;
  animation: d-button-spin 700ms linear infinite;
}

@keyframes d-button-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
