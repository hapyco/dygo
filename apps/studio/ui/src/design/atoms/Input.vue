<script setup lang="ts">
import type { ControlSize, TextInputType } from '../types'

withDefaults(defineProps<{
  id?: string
  modelValue?: string
  name?: string
  type?: TextInputType
  size?: ControlSize
  placeholder?: string
  autocomplete?: string
  describedBy?: string
  required?: boolean
  disabled?: boolean
  readonly?: boolean
  invalid?: boolean
}>(), {
  type: 'text',
  size: 'xs',
  readonly: false,
  invalid: false,
})

defineEmits<{
  'update:modelValue': [value: string]
}>()
</script>

<template>
  <input
    :id="id"
    class="d-input"
    :class="{ 'd-input--invalid': invalid }"
    :value="modelValue"
    :name="name"
    :type="type"
    :data-size="size"
    :placeholder="placeholder"
    :autocomplete="autocomplete"
    :required="required"
    :disabled="disabled"
    :readonly="readonly"
    :aria-invalid="invalid ? 'true' : undefined"
    :aria-describedby="describedBy || undefined"
    @input="$emit('update:modelValue', ($event.target as HTMLInputElement).value)"
  />
</template>

<style scoped>
.d-input {
  width: 100%;
  min-height: var(--studio-control-height-md);
  border: 1px solid var(--studio-border);
  border-radius: var(--studio-radius-control);
  background: var(--studio-control-bg);
  box-shadow: var(--studio-shadow-control);
  color: var(--studio-text);
  padding: 0 11px;
  font-size: 14px;
  line-height: 1;
  transition:
    border-color 160ms ease,
    box-shadow 160ms ease,
    background-color 160ms ease;
}

.d-input[data-size='sm'] {
  min-height: var(--studio-control-height-sm);
  padding: 0 9px;
  font-size: 13px;
}

.d-input[data-size='xs'] {
  min-height: var(--studio-control-height-xs);
  padding: 0 8px;
  font-size: 13px;
}

.d-input::placeholder {
  color: var(--studio-text-subtle);
}

.d-input:hover:not(:disabled) {
  background: var(--studio-control-bg-hover);
  border-color: var(--studio-border-strong);
}

.d-input:focus {
  outline: none;
}

.d-input:focus-visible {
  border-color: var(--studio-focus);
  box-shadow: var(--studio-focus-ring);
}

.d-input:disabled {
  background: var(--studio-control-bg-disabled);
  color: var(--studio-text-subtle);
}

.d-input:read-only:not(:disabled) {
  background: var(--studio-control-bg-readonly);
  color: var(--studio-text-muted);
}

.d-input--invalid {
  border-color: var(--studio-danger);
}

.d-input--invalid:focus-visible {
  border-color: var(--studio-danger);
  box-shadow: var(--studio-danger-ring);
}
</style>
