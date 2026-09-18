<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'

import {
  PasswordField,
  SelectField,
  SwitchField,
  TextareaField,
  TextField,
} from '@/design'
import {
  linkOptions,
  type MetadataEntityMeta,
  type MetadataField,
  type MetadataFormItem,
  type MetadataFormLayout,
  type MetadataFormTab,
} from '@/features/metadata/metadata.api'
import { useMetadataEntitiesQuery } from '@/features/metadata/metadata.query'
import { iconForEntity } from '@/features/metadata/entity-icons'
import { uploadRecordFile, type RecordData } from '@/features/records/records.api'
import { isHiddenRecordFormField, recordFieldLabel } from '@/features/records/system-fields'
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
  form?: MetadataFormLayout | null
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
  form: null,
  record: null,
  fieldErrors: () => ({}),
  disabled: false,
})

const emit = defineEmits<{
  'update:modelValue': [value: RecordData]
}>()

const router = useRouter()
const entitiesQuery = useMetadataEntitiesQuery()
const activeTabKey = ref('')

const visibleFields = computed(() => props.fields.filter((field) => !isHiddenRecordFormField(field.name, props.systemFields ?? [])))
const fieldsByName = computed(() => {
  const map = new Map<string, MetadataField>()
  for (const field of visibleFields.value) {
    map.set(field.name, field)
  }
  return map
})

const formLayout = computed<MetadataFormLayout>(() => {
  const tabs = props.form?.tabs?.filter((tab) => tab.items?.length) ?? []
  if (tabs.length > 0) {
    return { tabs }
  }
  return {
    tabs: [{
      key: 'default',
      label: props.entityLabel,
      items: visibleFields.value.map((field) => ({ kind: 'field' as const, name: field.name })),
    }],
  }
})

const showTabStrip = computed(() => formLayout.value.tabs.length > 1)

watch(
  formLayout,
  (layout) => {
    if (!layout.tabs.some((tab) => tab.key === activeTabKey.value)) {
      activeTabKey.value = layout.tabs[0]?.key ?? ''
    }
  },
  { immediate: true },
)

const activeTab = computed(() => (
  formLayout.value.tabs.find((tab) => tab.key === activeTabKey.value) ?? formLayout.value.tabs[0] ?? null
))

function tabHasErrors(tab: MetadataFormTab): boolean {
  return tab.items.some((item) => item.kind === 'field' && item.name && props.fieldErrors[item.name])
}

function columnsFor(tab: MetadataFormTab): MetadataFormItem[][] {
  const columns: MetadataFormItem[][] = [[]]
  for (const item of tab.items ?? []) {
    if (item.kind === 'column') {
      columns.push([])
      continue
    }
    columns[columns.length - 1]?.push(item)
  }
  return columns.filter((column) => column.length > 0)
}

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
  <form
    class="record-form-renderer"
    :class="{ 'record-form-renderer--wide': showTabStrip || (activeTab && columnsFor(activeTab).length > 1) }"
    :aria-label="`${entityLabel} form`"
  >
    <div
      v-if="showTabStrip"
      class="record-form-renderer__tabs"
      role="tablist"
      :aria-label="`${entityLabel} sections`"
    >
      <button
        v-for="tab in formLayout.tabs"
        :key="tab.key"
        type="button"
        class="record-form-renderer__tab"
        :class="{
          'record-form-renderer__tab--active': tab.key === activeTabKey,
          'record-form-renderer__tab--error': tabHasErrors(tab),
        }"
        role="tab"
        :aria-selected="tab.key === activeTabKey ? 'true' : 'false'"
        :id="`form-tab-${tab.key}`"
        :aria-controls="`form-panel-${tab.key}`"
        @click="activeTabKey = tab.key"
      >
        <component
          :is="iconForEntity(tab.icon)"
          v-if="tab.icon"
          class="record-form-renderer__tab-icon"
          aria-hidden="true"
        />
        <span>{{ tab.label }}</span>
      </button>
    </div>

    <div
      v-for="tab in formLayout.tabs"
      v-show="tab.key === activeTabKey"
      :id="`form-panel-${tab.key}`"
      :key="tab.key"
      class="record-form-renderer__panel"
      role="tabpanel"
      :aria-labelledby="showTabStrip ? `form-tab-${tab.key}` : undefined"
    >
      <div
        class="record-form-renderer__columns"
        :style="{ '--form-columns': String(Math.max(columnsFor(tab).length, 1)) }"
      >
        <div
          v-for="(column, columnIndex) in columnsFor(tab)"
          :key="`${tab.key}-${columnIndex}`"
          class="record-form-renderer__column"
        >
          <template v-for="(item, itemIndex) in column" :key="`${tab.key}-${columnIndex}-${itemIndex}`">
            <header
              v-if="item.kind === 'section'"
              class="record-form-renderer__section"
            >
              <h3 class="record-form-renderer__section-title">{{ item.label || 'Section' }}</h3>
              <p v-if="item.description" class="record-form-renderer__section-description">{{ item.description }}</p>
            </header>

            <template v-else-if="item.kind === 'field' && item.name && fieldsByName.get(item.name)">
              <template v-for="field in [fieldsByName.get(item.name)!]" :key="field.name">
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
            </template>
          </template>
        </div>
      </div>
    </div>
  </form>
</template>

<style scoped>
.record-form-renderer {
  display: grid;
  width: min(100%, 680px);
  gap: 16px;
  padding: 16px 0 24px;
}

.record-form-renderer--wide {
  width: min(100%, 960px);
}

.record-form-renderer__tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 18px;
  border-bottom: 1px solid var(--studio-border);
}

.record-form-renderer__tab {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin: 0;
  border: 0;
  border-bottom: 2px solid transparent;
  appearance: none;
  background: transparent;
  color: var(--studio-text-muted);
  cursor: pointer;
  font: inherit;
  font-size: 13px;
  font-weight: 500;
  line-height: 1.2;
  padding: 8px 2px 10px;
}

.record-form-renderer__tab-icon {
  width: 14px;
  height: 14px;
  flex: 0 0 auto;
}

.record-form-renderer__tab:hover {
  color: var(--studio-text);
}

.record-form-renderer__tab--active {
  border-bottom-color: var(--studio-text);
  color: var(--studio-text);
  font-weight: 700;
}

.record-form-renderer__tab--error {
  color: var(--studio-danger, #b42318);
}

.record-form-renderer__panel {
  min-width: 0;
}

.record-form-renderer__columns {
  display: grid;
  grid-template-columns: repeat(var(--form-columns, 1), minmax(0, 1fr));
  gap: 16px 24px;
}

.record-form-renderer__column {
  display: grid;
  gap: 14px;
  align-content: start;
  min-width: 0;
}

.record-form-renderer__section {
  display: grid;
  gap: 4px;
  padding-top: 4px;
}

.record-form-renderer__section-title {
  margin: 0;
  color: var(--studio-text);
  font-size: 14px;
  font-weight: 700;
  line-height: 1.3;
}

.record-form-renderer__section-description {
  margin: 0;
  color: var(--studio-text-muted);
  font-size: 12px;
  line-height: 1.4;
}

@media (max-width: 720px) {
  .record-form-renderer__columns {
    grid-template-columns: 1fr;
  }
}
</style>
