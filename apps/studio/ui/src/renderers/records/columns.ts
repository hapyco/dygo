import type { DataTableColumn } from '@/design/types'
import type { MetadataField } from '@/features/metadata/metadata.api'

export type RecordListColumnSource = 'name' | 'field' | 'system'

export type RecordListColumn = DataTableColumn & {
  source: RecordListColumnSource
  cellType: 'text'
  field?: MetadataField
}

export function buildRecordListColumns(fields: MetadataField[]): RecordListColumn[] {
  const seen = new Set<string>()
  const columns: RecordListColumn[] = [
    { key: 'name', label: 'Name', source: 'name', cellType: 'text', sortable: true },
    ...fields.map((field) => ({
      key: field.name,
      label: field.label || field.name,
      source: 'field' as const,
      cellType: 'text' as const,
      sortable: true,
      field,
    })),
    { key: 'created-at', label: 'Created At', source: 'system', cellType: 'text', sortable: true },
    { key: 'updated-at', label: 'Updated At', source: 'system', cellType: 'text', sortable: true },
  ]

  return columns.filter((column) => {
    if (column.key === 'id' || seen.has(column.key)) {
      return false
    }

    seen.add(column.key)
    return true
  })
}
