<script setup lang="ts">
import type { SegmentedControlOption, SegmentedControlValue } from '@/design/types'

const props = withDefaults(defineProps<{
  modelValue: SegmentedControlValue
  options: SegmentedControlOption[]
  disabled?: boolean
  ariaLabel?: string
}>(), {
  disabled: false,
  ariaLabel: 'Choose option',
})

const emit = defineEmits<{
  'update:modelValue': [value: SegmentedControlValue]
}>()

function selectOption(option: SegmentedControlOption) {
  if (props.disabled || option.disabled || option.value === props.modelValue) {
    return
  }

  emit('update:modelValue', option.value)
}
</script>

<template>
  <div
    class="segmented-control"
    role="radiogroup"
    :aria-label="ariaLabel"
    :aria-disabled="disabled ? 'true' : undefined"
  >
    <button
      v-for="option in options"
      :key="String(option.value)"
      type="button"
      class="segmented-control__item"
      :class="{ 'segmented-control__item--active': option.value === modelValue }"
      role="radio"
      :aria-checked="option.value === modelValue ? 'true' : 'false'"
      :disabled="disabled || option.disabled"
      @click="selectOption(option)"
    >
      {{ option.label }}
    </button>
  </div>
</template>

<style scoped>
.segmented-control {
  display: inline-flex;
  min-height: var(--studio-control-height-xs);
  overflow: hidden;
  border: 1px solid var(--studio-border);
  border-radius: var(--studio-radius-control);
  background: var(--studio-surface-raised);
}

.segmented-control__item {
  min-width: 48px;
  border: 0;
  border-left: 1px solid var(--studio-border);
  appearance: none;
  background: transparent;
  color: var(--studio-text-muted);
  cursor: pointer;
  font: inherit;
  font-size: 13px;
  font-weight: 500;
  line-height: 1;
  padding: 0 12px;
}

.segmented-control__item:first-child {
  border-left: 0;
}

.segmented-control__item:hover:not(:disabled) {
  background: var(--studio-control-bg-hover);
  color: var(--studio-text);
}

.segmented-control__item:focus-visible {
  position: relative;
  z-index: 1;
  outline: none;
  box-shadow: var(--studio-focus-ring);
}

.segmented-control__item--active {
  background: var(--studio-surface);
  color: var(--studio-text);
}

.segmented-control__item:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}
</style>
