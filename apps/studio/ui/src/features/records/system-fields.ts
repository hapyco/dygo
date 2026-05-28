import type { MetadataField } from '@/features/metadata/metadata.api'

const formHiddenSystemFields = new Set(['id', 'created-at', 'updated-at'])
const submitHiddenSystemFields = new Set(['id', 'created-at', 'updated-at'])

export function isRecordSystemField(name: string, systemFields: MetadataField[]): boolean {
  return systemFields.some((field) => field.name === name)
}

export function isHiddenRecordFormField(name: string, systemFields: MetadataField[]): boolean {
  return isRecordSystemField(name, systemFields) && formHiddenSystemFields.has(name)
}

export function isHiddenRecordSubmitField(name: string, systemFields: MetadataField[]): boolean {
  return isRecordSystemField(name, systemFields) && submitHiddenSystemFields.has(name)
}

export function recordSystemListColumns(fields: MetadataField[] = []) {
  return fields
    .filter((field) => field.name !== 'id' && field.listable && !field['write-only'])
    .map((field) => ({
      key: field.name,
      label: field.name === 'name' ? 'ID' : field.label || field.name,
      source: field.name === 'name' ? 'name' as const : 'system' as const,
      cellType: field.studio?.display || 'text',
      sortable: true,
      field,
    }))
}
