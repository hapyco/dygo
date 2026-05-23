import type { MetadataField } from '@/features/metadata/metadata.api'

export const recordSystemFields = [
  { name: 'id', label: 'ID', listColumn: false, formHidden: true, submitHidden: true },
  { name: 'name', label: 'Name', listColumn: true, formHidden: false, submitHidden: false },
  { name: 'created-at', label: 'Created At', listColumn: true, formHidden: true, submitHidden: true },
  { name: 'updated-at', label: 'Updated At', listColumn: true, formHidden: true, submitHidden: true },
] as const

export type RecordSystemFieldName = typeof recordSystemFields[number]['name']

export function isRecordSystemField(name: string): name is RecordSystemFieldName {
  return recordSystemFields.some((field) => field.name === name)
}

export function isHiddenRecordFormField(name: string): boolean {
  return recordSystemFields.some((field) => field.name === name && field.formHidden)
}

export function isHiddenRecordSubmitField(name: string): boolean {
  return recordSystemFields.some((field) => field.name === name && field.submitHidden)
}

export function recordSystemListColumns(fields: MetadataField[] = []) {
  if (fields.length > 0) {
    return fields
      .filter((field) => field.name !== 'id' && field.listable && !field['write-only'])
      .map((field) => ({
      key: field.name,
      label: field.label || field.name,
      source: field.name === 'name' ? 'name' as const : 'system' as const,
      cellType: field.studio?.display || 'text',
      sortable: true,
      field,
      }))
  }

  return recordSystemFields
    .filter((field) => field.listColumn)
    .map((field) => ({
      key: field.name,
      label: field.label,
      source: field.name === 'name' ? 'name' as const : 'system' as const,
      cellType: 'text',
      sortable: true,
    }))
}
