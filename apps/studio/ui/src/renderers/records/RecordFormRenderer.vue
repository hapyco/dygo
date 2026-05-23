<script setup lang="ts">
import { computed } from 'vue'

import {
  PasswordField,
  SelectField,
  SwitchField,
  TextareaField,
  TextField,
  type FieldOption,
  type TextInputType,
} from '@/design'
import type { MetadataField } from '@/features/metadata/metadata.api'
import type { RecordData } from '@/features/records/records.api'
import { isHiddenRecordFormField } from '@/features/records/system-fields'

const props = withDefaults(defineProps<{
  entity: string
  entityLabel: string
  fields: MetadataField[]
  systemFields?: MetadataField[]
  record?: RecordData | null
  mode: 'new' | 'record' | 'single'
  modelValue: RecordData
  fieldErrors?: Record<string, string>
  disabled?: boolean
}>(), {
  record: null,
  fieldErrors: () => ({}),
  disabled: false,
})

const emit = defineEmits<{
  'update:modelValue': [value: RecordData]
}>()

const visibleFields = computed(() => props.fields.filter((field) => !isHiddenRecordFormField(field.name, props.systemFields ?? [])))

function updateField(field: MetadataField, value: unknown) {
  emit('update:modelValue', {
    ...props.modelValue,
    [field.name]: value,
  })
}

function fieldId(field: MetadataField): string {
  return `record-${props.entity}-${field.name}`.replace(/[^a-zA-Z0-9_-]+/g, '-')
}

function labelForField(field: MetadataField): string {
  return field.label || field.name
}

function isReadonlyField(field: MetadataField): boolean {
  return props.mode !== 'new' && field.name === 'name'
}

function textValue(field: MetadataField): string {
  const value = props.modelValue[field.name]
  if (value === null || value === undefined) {
    return ''
  }

  if (typeof value === 'string') {
    return value
  }

  if (typeof value === 'number' || typeof value === 'bigint' || typeof value === 'boolean') {
    return String(value)
  }

  return JSON.stringify(value, null, 2)
}

function booleanValue(field: MetadataField): boolean {
  return props.modelValue[field.name] === true
}

function inputTypeForField(field: MetadataField): Exclude<TextInputType, 'password'> {
  switch (editorForField(field)) {
    case 'email':
      return 'email'
    case 'date':
      return 'date'
    case 'number':
      return 'number'
    default:
      return 'text'
  }
}

function editorForField(field: MetadataField): string {
  return field.studio?.editor || field.type
}

function isTextField(field: MetadataField): boolean {
  return ['text', 'email', 'number', 'link', 'date', 'datetime', 'time'].includes(editorForField(field))
}

function isTextareaField(field: MetadataField): boolean {
  return editorForField(field) === 'textarea' || editorForField(field) === 'json'
}

function selectOptions(field: MetadataField): FieldOption[] {
  const options = field.options
  if (!options || typeof options !== 'object' || !('values' in options)) {
    return []
  }

  const values = (options as { values?: unknown }).values
  if (!Array.isArray(values)) {
    return []
  }

  return values
    .filter((value): value is string | number => typeof value === 'string' || typeof value === 'number')
    .map((value) => ({ value: String(value), label: String(value) }))
}
</script>

<template>
  <form class="record-form-renderer" :aria-label="`${entityLabel} form`">
    <template v-for="field in visibleFields" :key="field.name">
      <PasswordField
        v-if="editorForField(field) === 'password'"
        :id="fieldId(field)"
        :label="labelForField(field)"
        :model-value="textValue(field)"
        :name="field.name"
        :required="mode === 'new' && field.required"
        :disabled="disabled"
        :readonly="isReadonlyField(field)"
        :error="fieldErrors[field.name]"
        :placeholder="mode === 'record' ? 'Leave blank to keep unchanged' : undefined"
        autocomplete="new-password"
        @update:model-value="updateField(field, $event)"
      />

      <SwitchField
        v-else-if="editorForField(field) === 'switch'"
        :id="fieldId(field)"
        :label="labelForField(field)"
        :model-value="booleanValue(field)"
        :name="field.name"
        :required="field.required"
        :disabled="disabled"
        :readonly="isReadonlyField(field)"
        :error="fieldErrors[field.name]"
        @update:model-value="updateField(field, $event)"
      />

      <SelectField
        v-else-if="editorForField(field) === 'select'"
        :id="fieldId(field)"
        :label="labelForField(field)"
        :model-value="textValue(field)"
        :name="field.name"
        :options="selectOptions(field)"
        :required="field.required"
        :disabled="disabled"
        :readonly="isReadonlyField(field)"
        :error="fieldErrors[field.name]"
        placeholder="Select"
        @update:model-value="updateField(field, $event)"
      />

      <TextareaField
        v-else-if="isTextareaField(field)"
        :id="fieldId(field)"
        :label="labelForField(field)"
        :model-value="textValue(field)"
        :name="field.name"
        :required="field.required"
        :disabled="disabled"
        :readonly="isReadonlyField(field)"
        :error="fieldErrors[field.name]"
        :rows="editorForField(field) === 'json' ? 7 : 4"
        @update:model-value="updateField(field, $event)"
      />

      <TextField
        v-else-if="isTextField(field)"
        :id="fieldId(field)"
        :label="labelForField(field)"
        :model-value="textValue(field)"
        :name="field.name"
        :type="inputTypeForField(field)"
        :required="field.required"
        :disabled="disabled"
        :readonly="isReadonlyField(field)"
        :error="fieldErrors[field.name]"
        @update:model-value="updateField(field, $event)"
      />

      <TextareaField
        v-else
        :id="fieldId(field)"
        :label="labelForField(field)"
        :model-value="textValue(field)"
        :name="field.name"
        readonly
        :disabled="disabled"
        :hint="`Field type ${field.type} is not editable yet.`"
        :rows="3"
      />
    </template>
  </form>
</template>

<style scoped>
.record-form-renderer {
  display: grid;
  width: min(100%, 680px);
  gap: 14px;
  padding: 16px 0 24px;
}
</style>
