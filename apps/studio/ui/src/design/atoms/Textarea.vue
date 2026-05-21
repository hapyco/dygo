<script setup lang="ts">
import type { ControlSize } from '../types'

withDefaults(defineProps<{
  id?: string
  modelValue?: string
  name?: string
  size?: ControlSize
  placeholder?: string
  describedBy?: string
  rows?: number
  required?: boolean
  disabled?: boolean
  readonly?: boolean
  invalid?: boolean
}>(), {
  size: 'md',
  rows: 4,
  readonly: false,
  invalid: false,
})

defineEmits<{
  'update:modelValue': [value: string]
}>()
</script>

<template>
  <textarea
    :id="id"
    class="d-textarea"
    :class="{ 'd-textarea--invalid': invalid }"
    :value="modelValue"
    :name="name"
    :data-size="size"
    :placeholder="placeholder"
    :rows="rows"
    :required="required"
    :disabled="disabled"
    :readonly="readonly"
    :aria-invalid="invalid ? 'true' : undefined"
    :aria-describedby="describedBy || undefined"
    @input="$emit('update:modelValue', ($event.target as HTMLTextAreaElement).value)"
  />
</template>

<style scoped>
.d-textarea {
  width: 100%;
  min-height: 112px;
  resize: vertical;
  border: 1px solid var(--studio-border);
  border-radius: var(--studio-radius-control);
  background: var(--studio-control-bg);
  box-shadow: var(--studio-shadow-control);
  color: var(--studio-text);
  padding: 10px 11px;
  font-size: 14px;
  line-height: 1.45;
  transition:
    border-color 160ms ease,
    box-shadow 160ms ease,
    background-color 160ms ease;
}

.d-textarea[data-size='sm'] {
  min-height: 88px;
  padding: 8px 9px;
  font-size: 13px;
}

.d-textarea[data-size='xs'] {
  min-height: 72px;
  padding: 7px 8px;
  font-size: 13px;
}

.d-textarea::placeholder {
  color: var(--studio-text-subtle);
}

.d-textarea:hover:not(:disabled) {
  background: var(--studio-control-bg-hover);
  border-color: var(--studio-border-strong);
}

.d-textarea:focus {
  outline: none;
}

.d-textarea:focus-visible {
  border-color: var(--studio-focus);
  box-shadow: var(--studio-focus-ring);
}

.d-textarea:disabled {
  background: var(--studio-control-bg-disabled);
  color: var(--studio-text-subtle);
}

.d-textarea:read-only:not(:disabled) {
  background: var(--studio-control-bg-readonly);
  color: var(--studio-text-muted);
}

.d-textarea--invalid {
  border-color: var(--studio-danger);
}

.d-textarea--invalid:focus-visible {
  border-color: var(--studio-danger);
  box-shadow: var(--studio-danger-ring);
}
</style>
