<script setup lang="ts">
import RadioGroup from '../primitives/RadioGroup.vue'
import type { FieldOption } from '../types'
import Field from './Field.vue'

withDefaults(defineProps<{
  id: string
  label: string
  modelValue?: string
  name?: string
  options: FieldOption[]
  orientation?: 'horizontal' | 'vertical'
  hint?: string
  error?: string
  required?: boolean
  disabled?: boolean
  readonly?: boolean
}>(), {
  orientation: 'vertical',
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
    <template #default="{ invalid, describedBy }">
      <RadioGroup
        :id="id"
        :model-value="modelValue"
        :name="name"
        :options="options"
        :orientation="orientation"
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
