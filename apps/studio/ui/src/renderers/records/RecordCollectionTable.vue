<script setup lang="ts">
import { computed } from 'vue'
import { ArrowDown, ArrowUp, Plus, Trash2 } from '@lucide/vue'

import { Button, IconButton, Input, Select, Switch, Textarea, type FieldOption, type TextInputType } from '@/design'
import type { MetadataEntityMeta, MetadataField } from '@/features/metadata/metadata.api'
import type { RecordData } from '@/features/records/records.api'

const props = withDefaults(defineProps<{
  id: string
  label: string
  field: MetadataField
  childMeta?: MetadataEntityMeta
  modelValue?: unknown
  error?: string
  required?: boolean
  disabled?: boolean
}>(), {
  childMeta: undefined,
  modelValue: () => [],
  error: '',
  required: false,
  disabled: false,
})

const emit = defineEmits<{
  'update:modelValue': [value: RecordData[]]
}>()

const rows = computed<RecordData[]>(() => {
  if (!Array.isArray(props.modelValue)) {
    return []
  }

  return props.modelValue.filter((row): row is RecordData => Boolean(row) && typeof row === 'object' && !Array.isArray(row))
})

const columns = computed(() => {
  const fields = props.childMeta?.fields ?? []
  return fields.filter((field) => !isHiddenCollectionField(field))
})

function addRow() {
  emitRows([...rows.value, newRow()])
}

function deleteRow(index: number) {
  const next = rows.value.slice()
  next.splice(index, 1)
  emitRows(next)
}

function moveRow(index: number, direction: -1 | 1) {
  const target = index + direction
  if (target < 0 || target >= rows.value.length) {
    return
  }

  const next = rows.value.slice()
  const [row] = next.splice(index, 1)
  next.splice(target, 0, row)
  emitRows(next)
}

function updateCell(index: number, field: MetadataField, value: unknown) {
  const next = rows.value.map((row, rowIndex) => {
    if (rowIndex !== index) {
      return row
    }

    return {
      ...row,
      [field.name]: value,
    }
  })
  emitRows(next)
}

function emitRows(next: RecordData[]) {
  emit('update:modelValue', next)
}

function newRow(): RecordData {
  return columns.value.reduce<RecordData>((row, field) => {
    row[field.name] = initialFieldValue(field)
    return row
  }, {})
}

function initialFieldValue(field: MetadataField): unknown {
  if (field.default !== undefined) {
    return cloneDefault(field.default)
  }
  if (field['value-kind'] === 'boolean') {
    return false
  }
  return ''
}

function cloneDefault(value: unknown): unknown {
  if (value === null || typeof value !== 'object') {
    return value
  }

  return JSON.parse(JSON.stringify(value))
}

function rowKey(row: RecordData, index: number): string {
  const id = row.id
  if (typeof id === 'string' || typeof id === 'number') {
    return `row-${id}`
  }
  return `draft-${index}`
}

function fieldId(field: MetadataField, index: number): string {
  return `${props.id}-${index}-${field.name}`.replace(/[^a-zA-Z0-9_-]+/g, '-')
}

function labelForField(field: MetadataField): string {
  return field.label || field.name
}

function textValue(row: RecordData, field: MetadataField): string {
  const value = row[field.name]
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

function booleanValue(row: RecordData, field: MetadataField): boolean {
  return row[field.name] === true
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
  return ['text', 'email', 'number', 'link', 'date', 'datetime', 'time', 'password'].includes(editorForField(field))
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

function isHiddenCollectionField(field: MetadataField): boolean {
  if (field.type === 'collection') {
    return true
  }

  return [
    'id',
    'name',
    'created-at',
    'updated-at',
    'parent-entity-id',
    'parent_entity_id',
    'parent-record-id',
    'parent_record_id',
    'parent-field-id',
    'parent_field_id',
    'ordinal',
  ].includes(field.name)
}
</script>

<template>
  <section class="record-collection-table" :aria-labelledby="`${id}-label`">
    <div class="record-collection-table__header">
      <label :id="`${id}-label`" class="record-collection-table__label">
        {{ label }}
        <span v-if="required" class="record-collection-table__required" aria-hidden="true">*</span>
      </label>
    </div>

    <div class="record-collection-table__frame" :class="{ 'record-collection-table__frame--invalid': Boolean(error) }">
      <table class="record-collection-table__table">
        <thead>
          <tr>
            <th v-for="column in columns" :key="column.name" scope="col">
              {{ labelForField(column) }}
              <span v-if="column.required" class="record-collection-table__required" aria-hidden="true">*</span>
            </th>
            <th class="record-collection-table__actions" scope="col"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(row, rowIndex) in rows" :key="rowKey(row, rowIndex)">
            <td v-for="column in columns" :key="column.name">
              <Select
                v-if="editorForField(column) === 'select'"
                :id="fieldId(column, rowIndex)"
                :model-value="textValue(row, column)"
                :name="`${field.name}.${rowIndex}.${column.name}`"
                :options="selectOptions(column)"
                size="sm"
                :disabled="disabled"
                :invalid="Boolean(error)"
                @update:model-value="updateCell(rowIndex, column, $event)"
              />

              <Switch
                v-else-if="editorForField(column) === 'switch'"
                :id="fieldId(column, rowIndex)"
                :model-value="booleanValue(row, column)"
                :name="`${field.name}.${rowIndex}.${column.name}`"
                :disabled="disabled"
                :invalid="Boolean(error)"
                @update:model-value="updateCell(rowIndex, column, $event)"
              />

              <Textarea
                v-else-if="isTextareaField(column)"
                :id="fieldId(column, rowIndex)"
                :model-value="textValue(row, column)"
                :name="`${field.name}.${rowIndex}.${column.name}`"
                size="xs"
                :rows="2"
                :disabled="disabled"
                :invalid="Boolean(error)"
                @update:model-value="updateCell(rowIndex, column, $event)"
              />

              <Input
                v-else-if="isTextField(column)"
                :id="fieldId(column, rowIndex)"
                :model-value="textValue(row, column)"
                :name="`${field.name}.${rowIndex}.${column.name}`"
                :type="editorForField(column) === 'password' ? 'password' : inputTypeForField(column)"
                size="sm"
                :disabled="disabled"
                :invalid="Boolean(error)"
                @update:model-value="updateCell(rowIndex, column, $event)"
              />

              <Input
                v-else
                :id="fieldId(column, rowIndex)"
                :model-value="textValue(row, column)"
                :name="`${field.name}.${rowIndex}.${column.name}`"
                size="sm"
                readonly
                :disabled="disabled"
                :invalid="Boolean(error)"
              />
            </td>
            <td class="record-collection-table__actions">
              <div class="record-collection-table__action-row">
                <IconButton
                  label="Move row up"
                  variant="ghost"
                  size="xs"
                  :disabled="disabled || rowIndex === 0"
                  @click="moveRow(rowIndex, -1)"
                >
                  <ArrowUp :size="15" :stroke-width="1.8" />
                </IconButton>
                <IconButton
                  label="Move row down"
                  variant="ghost"
                  size="xs"
                  :disabled="disabled || rowIndex === rows.length - 1"
                  @click="moveRow(rowIndex, 1)"
                >
                  <ArrowDown :size="15" :stroke-width="1.8" />
                </IconButton>
                <IconButton
                  label="Delete row"
                  variant="danger"
                  size="xs"
                  :disabled="disabled"
                  @click="deleteRow(rowIndex)"
                >
                  <Trash2 :size="15" :stroke-width="1.8" />
                </IconButton>
              </div>
            </td>
          </tr>
          <tr v-if="rows.length === 0">
            <td class="record-collection-table__empty" :colspan="Math.max(columns.length + 1, 1)">No rows</td>
          </tr>
        </tbody>
      </table>
    </div>

    <p v-if="error" class="record-collection-table__error" role="alert">{{ error }}</p>

    <Button variant="secondary" size="sm" :disabled="disabled || columns.length === 0" @click="addRow">
      <Plus :size="15" :stroke-width="1.8" />
      Add row
    </Button>
  </section>
</template>

<style scoped>
.record-collection-table {
  display: grid;
  gap: 8px;
}

.record-collection-table__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.record-collection-table__label {
  color: var(--studio-text-muted);
  font-size: 13px;
  font-weight: 600;
  line-height: 1.3;
}

.record-collection-table__required {
  color: var(--studio-danger);
}

.record-collection-table__frame {
  max-width: 100%;
  overflow-x: auto;
  border: 1px solid var(--studio-border);
  border-radius: var(--studio-radius-panel);
  background: var(--studio-surface);
}

.record-collection-table__frame--invalid {
  border-color: var(--studio-danger);
}

.record-collection-table__table {
  width: 100%;
  min-width: 540px;
  border-collapse: collapse;
  table-layout: fixed;
}

.record-collection-table__table th,
.record-collection-table__table td {
  border-right: 1px solid var(--studio-border);
  border-bottom: 1px solid var(--studio-border);
  padding: 8px;
  vertical-align: top;
}

.record-collection-table__table th {
  background: var(--studio-surface-raised);
  color: var(--studio-text-muted);
  font-size: 12px;
  font-weight: 700;
  line-height: 1.2;
  text-align: left;
}

.record-collection-table__table td {
  background: var(--studio-surface);
}

.record-collection-table__table tr:last-child td {
  border-bottom: 0;
}

.record-collection-table__table th:last-child,
.record-collection-table__table td:last-child {
  border-right: 0;
}

.record-collection-table__actions {
  width: 112px;
}

.record-collection-table__action-row {
  display: flex;
  justify-content: flex-end;
  gap: 4px;
}

.record-collection-table__empty {
  color: var(--studio-text-subtle);
  font-size: 13px;
  font-weight: 500;
  text-align: center;
}

.record-collection-table__error {
  margin: 0;
  color: var(--studio-danger);
  font-size: 12px;
  line-height: 1.35;
}
</style>
