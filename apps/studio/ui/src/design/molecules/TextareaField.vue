<script setup lang="ts">
import Textarea from '../atoms/Textarea.vue'
import type { ControlSize } from '../types'
import Field from './Field.vue'

withDefaults(defineProps<{
  id: string
  label: string
  modelValue?: string
  name?: string
  size?: ControlSize
  placeholder?: string
  rows?: number
  hint?: string
  error?: string
  required?: boolean
  disabled?: boolean
  readonly?: boolean
}>(), {
  size: 'md',
  rows: 4,
  required: false,
  disabled: false,
  readonly: false,
})

defineEmits<{
  'update:modelValue': [value: string]
}>()
</script>

<template>
  <Field
    :id="id"
    :label="label"
    :hint="hint"
    :error="error"
    :required="required"
    :disabled="disabled"
    :readonly="readonly"
  >
    <template #default="{ id: fieldId, invalid, describedBy }">
      <Textarea
        :id="fieldId"
        :model-value="modelValue"
        :name="name"
        :size="size"
        :placeholder="placeholder"
        :rows="rows"
        :described-by="describedBy"
        :required="required"
        :disabled="disabled"
        :readonly="readonly"
        :invalid="invalid"
        @update:model-value="$emit('update:modelValue', $event)"
      />
    </template>
  </Field>
</template>
