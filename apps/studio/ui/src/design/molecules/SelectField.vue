<script setup lang="ts">
import Select from '../primitives/Select.vue'
import type { ControlSize, FieldOption } from '../types'
import Field from './Field.vue'

withDefaults(defineProps<{
  id: string
  label: string
  modelValue?: string
  name?: string
  options: FieldOption[]
  placeholder?: string
  size?: ControlSize
  hint?: string
  error?: string
  required?: boolean
  disabled?: boolean
  readonly?: boolean
}>(), {
  size: 'md',
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
      <Select
        :id="fieldId"
        :model-value="modelValue"
        :name="name"
        :options="options"
        :placeholder="placeholder"
        :size="size"
        :described-by="describedBy"
        :disabled="disabled"
        :readonly="readonly"
        :invalid="invalid"
        @update:model-value="$emit('update:modelValue', $event)"
      />
    </template>
  </Field>
</template>
