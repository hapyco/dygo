<script setup lang="ts">
import { computed } from 'vue'

import Checkbox from '../atoms/Checkbox.vue'

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
    class="d-checkbox-field"
    :class="{
      'd-checkbox-field--disabled': disabled,
      'd-checkbox-field--readonly': readonly,
      'd-checkbox-field--invalid': Boolean(error),
    }"
  >
    <label class="d-checkbox-field__label" :for="id">
      <Checkbox
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
      <span class="d-checkbox-field__text">
        {{ label }}
        <span v-if="required" class="d-checkbox-field__required" aria-hidden="true">*</span>
      </span>
    </label>
    <p v-if="hint" :id="hintId" class="d-checkbox-field__hint">{{ hint }}</p>
    <p v-if="error" :id="errorId" class="d-checkbox-field__error" role="alert">{{ error }}</p>
  </div>
</template>

<style scoped>
.d-checkbox-field {
  display: grid;
  gap: 6px;
}

.d-checkbox-field__label {
  display: inline-flex;
  width: fit-content;
  align-items: center;
  gap: 9px;
  color: var(--studio-text-muted);
  font-size: 13px;
  line-height: 1.3;
}

.d-checkbox-field__text {
  user-select: none;
}

.d-checkbox-field__required {
  color: var(--studio-danger);
}

.d-checkbox-field__hint,
.d-checkbox-field__error {
  margin: 0 0 0 25px;
  font-size: 12px;
  line-height: 1.35;
}

.d-checkbox-field__hint {
  color: var(--studio-text-subtle);
}

.d-checkbox-field__error {
  color: var(--studio-danger);
}

.d-checkbox-field--disabled .d-checkbox-field__label,
.d-checkbox-field--readonly .d-checkbox-field__label {
  color: var(--studio-text-subtle);
}
</style>
