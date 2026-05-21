<script setup lang="ts">
import { RadioGroupIndicator, RadioGroupItem, RadioGroupRoot } from 'reka-ui'

import type { FieldOption } from '../types'

const props = withDefaults(defineProps<{
  id?: string
  modelValue?: string
  name?: string
  options: FieldOption[]
  orientation?: 'horizontal' | 'vertical'
  describedBy?: string
  required?: boolean
  disabled?: boolean
  readonly?: boolean
  invalid?: boolean
}>(), {
  orientation: 'vertical',
  disabled: false,
  readonly: false,
  invalid: false,
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

function updateValue(value: unknown) {
  if (typeof value === 'string') {
    emit('update:modelValue', value)
  }
}

function optionId(value: string): string | undefined {
  if (!props.id) {
    return undefined
  }
  return `${props.id}-${value.replace(/[^a-zA-Z0-9_-]/g, '-')}`
}
</script>

<template>
  <RadioGroupRoot
    class="d-radio-group"
    :class="{ 'd-radio-group--invalid': invalid }"
    :model-value="modelValue"
    :name="name"
    :orientation="orientation"
    :required="required"
    :disabled="disabled || readonly"
    :aria-invalid="invalid ? 'true' : undefined"
    :aria-describedby="describedBy || undefined"
    :aria-readonly="readonly ? 'true' : undefined"
    :data-readonly="readonly ? '' : undefined"
    @update:model-value="updateValue"
  >
    <label
      v-for="option in options"
      :key="option.value"
      class="d-radio-option"
      :class="{ 'd-radio-option--disabled': disabled || readonly || option.disabled }"
      :for="optionId(option.value)"
    >
      <RadioGroupItem
        :id="optionId(option.value)"
        class="d-radio"
        :value="option.value"
        :disabled="option.disabled"
      >
        <RadioGroupIndicator class="d-radio__indicator" />
      </RadioGroupItem>
      <span class="d-radio-option__label">{{ option.label }}</span>
    </label>
  </RadioGroupRoot>
</template>

<style scoped>
.d-radio-group {
  display: flex;
  flex-direction: column;
  gap: 9px;
}

.d-radio-group[data-orientation='horizontal'] {
  flex-flow: row wrap;
  gap: 10px 16px;
}

.d-radio-option {
  display: inline-flex;
  width: fit-content;
  align-items: center;
  gap: 9px;
  color: var(--studio-text-muted);
  font-size: 13px;
  line-height: 1.3;
}

.d-radio-option--disabled {
  color: var(--studio-text-subtle);
}

.d-radio {
  display: inline-flex;
  width: 16px;
  height: 16px;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--studio-border-strong);
  border-radius: 999px;
  background: var(--studio-control-bg);
  box-shadow: var(--studio-shadow-control);
  transition:
    background-color 160ms ease,
    border-color 160ms ease,
    box-shadow 160ms ease;
}

.d-radio:hover:not([data-disabled]) {
  border-color: var(--studio-accent);
}

.d-radio:focus-visible {
  outline: 2px solid var(--studio-focus);
  outline-offset: 2px;
}

.d-radio[data-state='checked'] {
  border-color: var(--studio-accent);
}

.d-radio[data-disabled] {
  opacity: 0.58;
}

.d-radio-group[data-readonly] .d-radio {
  background: var(--studio-control-bg-readonly);
  opacity: 1;
}

.d-radio-group--invalid .d-radio {
  border-color: var(--studio-danger);
}

.d-radio__indicator {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: var(--studio-accent);
}

.d-radio-option__label {
  user-select: none;
}
</style>
