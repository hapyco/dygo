<script setup lang="ts">
import { computed } from 'vue'

import Switch from '../primitives/Switch.vue'

const props = withDefaults(defineProps<{
  id: string
  label: string
  modelValue?: boolean
  name?: string
  hint?: string
  error?: string
  required?: boolean
  disabled?: boolean
  readonly?: boolean
}>(), {
  modelValue: false,
  required: false,
  disabled: false,
  readonly: false,
})

defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const hintId = computed(() => (props.hint ? `${props.id}-hint` : undefined))
const errorId = computed(() => (props.error ? `${props.id}-error` : undefined))
const describedBy = computed(() => [hintId.value, errorId.value].filter(Boolean).join(' ') || undefined)
</script>

<template>
  <div
    class="d-switch-field"
    :class="{
      'd-switch-field--disabled': disabled,
      'd-switch-field--readonly': readonly,
      'd-switch-field--invalid': Boolean(error),
    }"
  >
    <div class="d-switch-field__row">
      <label class="d-switch-field__label" :for="id">
        {{ label }}
        <span v-if="required" class="d-switch-field__required" aria-hidden="true">*</span>
      </label>
      <Switch
        :id="id"
        :model-value="modelValue"
        :name="name"
        :described-by="describedBy"
        :required="required"
        :disabled="disabled"
        :readonly="readonly"
        :invalid="Boolean(error)"
        @update:model-value="$emit('update:modelValue', $event)"
      />
    </div>
    <p v-if="hint" :id="hintId" class="d-switch-field__hint">{{ hint }}</p>
    <p v-if="error" :id="errorId" class="d-switch-field__error" role="alert">{{ error }}</p>
  </div>
</template>

<style scoped>
.d-switch-field {
  display: grid;
  gap: 6px;
}

.d-switch-field__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.d-switch-field__label {
  color: var(--studio-text-muted);
  font-size: 13px;
  font-weight: 600;
  line-height: 1.3;
}

.d-switch-field__required {
  color: var(--studio-danger);
}

.d-switch-field__hint,
.d-switch-field__error {
  margin: 0;
  font-size: 12px;
  line-height: 1.35;
}

.d-switch-field__hint {
  color: var(--studio-text-subtle);
}

.d-switch-field__error {
  color: var(--studio-danger);
}

.d-switch-field--disabled .d-switch-field__label,
.d-switch-field--readonly .d-switch-field__label {
  color: var(--studio-text-subtle);
}
</style>
