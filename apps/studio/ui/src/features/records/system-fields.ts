import type { MetadataField } from '@/features/metadata/metadata.api'

const hiddenSystemFields = new Set(['id', 'created-at', 'updated-at'])
const hiddenCollectionFields = new Set([
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
])

export function isRecordSystemField(name: string, systemFields: MetadataField[]): boolean {
  return systemFields.some((field) => field.name === name)
}

export function isHiddenRecordField(name: string, systemFields: MetadataField[]): boolean {
  return isRecordSystemField(name, systemFields) && hiddenSystemFields.has(name)
}

export function isHiddenCollectionField(field: MetadataField): boolean {
  return field.type === 'collection' || hiddenCollectionFields.has(field.name)
}

export function recordFieldLabel(field: MetadataField): string {
  return field.label || field.name
}

export function recordSystemListColumns(fields: MetadataField[] = []) {
  return fields
    .filter((field) => !hiddenSystemFields.has(field.name) && field.listable && !field['write-only'])
    .map((field) => ({
      key: field.name,
      label: field.name === 'name' ? 'ID' : recordFieldLabel(field),
      source: field.name === 'name' ? 'name' as const : 'system' as const,
      cellType: field.studio?.display || 'text',
      sortable: true,
      field,
    }))
}
