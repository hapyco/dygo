<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'

import {
  PasswordField,
  SelectField,
  SwitchField,
  TextareaField,
  TextField,
} from '@/design'
import { linkOptions, type MetadataEntityMeta, type MetadataField } from '@/features/metadata/metadata.api'
import { useMetadataEntitiesQuery } from '@/features/metadata/metadata.query'
import { uploadRecordFile, type RecordData } from '@/features/records/records.api'
import { isHiddenRecordField, recordFieldLabel } from '@/features/records/system-fields'
import SecretEditor from './SecretEditor.vue'
import type { SecretStatus } from '@/features/records/records.api'
import RecordCollectionTable from './RecordCollectionTable.vue'
import LinkPicker from './LinkPicker.vue'
import AttachmentEditor from './AttachmentEditor.vue'
import { RouteName } from '@/router/routes'
import { booleanValue, editorForField, inputTypeForField, isTextareaField, isTextField, selectOptions, textValue } from './record-field-utils'

const props = withDefaults(defineProps<{
  entity: string
  appName: string
  entityKey: string
  entityLabel: string
  fields: MetadataField[]
  systemFields?: MetadataField[]
  collections?: Record<string, MetadataEntityMeta>
  tree?: MetadataEntityMeta['tree']
  secretStatus?: SecretStatus
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

const router = useRouter()
const entitiesQuery = useMetadataEntitiesQuery()

const visibleFields = computed(() => props.fields.filter((field) => !isHiddenRecordField(field.name, props.systemFields ?? [])))

function updateField(field: MetadataField, value: unknown) {
  emit('update:modelValue', {
    ...props.modelValue,
    [field.name]: value,
  })
}

function attachmentUpload(field: MetadataField) {
  const id = Number(props.record?.id)
  if (props.mode !== 'record' || !Number.isInteger(id) || id <= 0) return undefined
  return async (file: File) => String((await uploadRecordFile(props.appName, props.entityKey, id, field.name, file)).id)
}

function relatedEntityRoute(field: MetadataField): string {
  const options = linkOptions(field)
  if (!options) {
    return ''
  }
  return (entitiesQuery.data.value ?? []).find((entity) => (
    entity.key === options.entity && (!options.app || entity.app.name === options.app)
  ))?.slug || options.entity
}

function openRelated(field: MetadataField, recordName: string) {
  const entity = relatedEntityRoute(field)
  if (!entity || !recordName) {
    return
  }
  void router.push({ name: RouteName.RecordDetail, params: { entity, recordName } })
}

function createRelated(field: MetadataField) {
  const entity = relatedEntityRoute(field)
  if (!entity) {
    return
  }
  void router.push({ name: RouteName.RecordNew, params: { entity } })
}

function fieldId(field: MetadataField): string {
  return `record-${props.entity}-${field.name}`.replace(/[^a-zA-Z0-9_-]+/g, '-')
}

function isReadonlyField(field: MetadataField): boolean {
  return props.mode !== 'new' && field.name === 'name'
}

</script>

<template>
  <form class="record-form-renderer" :aria-label="`${entityLabel} form`">
    <template v-for="field in visibleFields" :key="field.name">
      <RecordCollectionTable
        v-if="editorForField(field) === 'collection'"
        :id="fieldId(field)"
        :label="recordFieldLabel(field)"
        :field="field"
        :child-meta="collections?.[field.name]"
        :secret-status="secretStatus?.collections?.[field.name]"
        :model-value="modelValue[field.name]"
        :required="field.required"
        :disabled="disabled"
        :error="fieldErrors[field.name]"
        @update:model-value="updateField(field, $event)"
        @open-related="openRelated($event.field, $event.recordName)"
        @create-related="createRelated($event)"
      />

      <SecretEditor
        v-else-if="field.type === 'secret'"
        :id="fieldId(field)"
        :label="recordFieldLabel(field)"
        :model-value="modelValue[field.name]"
        :present="mode === 'new' ? false : secretStatus?.fields[field.name]"
        :required="field.required"
        :disabled="disabled"
        :error="fieldErrors[field.name]"
        @update:model-value="updateField(field, $event)"
      />

      <PasswordField
        v-else-if="editorForField(field) === 'password'"
        :id="fieldId(field)"
        :label="recordFieldLabel(field)"
        :model-value="textValue(modelValue[field.name])"
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
        :label="recordFieldLabel(field)"
        :model-value="booleanValue(modelValue[field.name])"
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
        :label="recordFieldLabel(field)"
        :model-value="textValue(modelValue[field.name])"
        :name="field.name"
        :options="selectOptions(field)"
        :required="field.required"
        :disabled="disabled"
        :readonly="isReadonlyField(field)"
        :error="fieldErrors[field.name]"
        placeholder="Select"
        @update:model-value="updateField(field, $event)"
      />

      <LinkPicker
        v-else-if="editorForField(field) === 'link'"
        :id="fieldId(field)"
        :label="recordFieldLabel(field)"
        :field="field"
        :model-value="textValue(modelValue[field.name])"
        :current-values="modelValue"
        :exclude-subtree="tree?.['parent-field'] === field.name ? textValue(record?.name) : ''"
        :required="field.required"
        :disabled="disabled"
        :readonly="isReadonlyField(field)"
        :error="fieldErrors[field.name]"
        @update:model-value="updateField(field, $event)"
        @open-related="openRelated(field, $event)"
        @create-related="createRelated(field)"
      />

      <TextareaField
        v-else-if="isTextareaField(field)"
        :id="fieldId(field)"
        :label="recordFieldLabel(field)"
        :model-value="textValue(modelValue[field.name])"
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
        :label="recordFieldLabel(field)"
        :model-value="textValue(modelValue[field.name])"
        :name="field.name"
        :type="inputTypeForField(field)"
        :required="field.required"
        :disabled="disabled"
        :readonly="isReadonlyField(field)"
        :error="fieldErrors[field.name]"
        @update:model-value="updateField(field, $event)"
      />

      <AttachmentEditor
        v-else-if="field.type === 'attachment'"
        :id="fieldId(field)"
        :label="recordFieldLabel(field)"
        :model-value="textValue(modelValue[field.name])"
        :upload="attachmentUpload(field)"
        :disabled="disabled"
        :readonly="isReadonlyField(field)"
        :error="fieldErrors[field.name]"
        @update:model-value="updateField(field, $event)"
      />

      <TextareaField
        v-else
        :id="fieldId(field)"
        :label="recordFieldLabel(field)"
        :model-value="textValue(modelValue[field.name])"
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
