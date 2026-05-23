import type { DataTableColumn } from '@/design/types'
import type { MetadataField } from '@/features/metadata/metadata.api'
import { recordSystemListColumns } from '../../features/records/system-fields.ts'

export type RecordListColumnSource = 'name' | 'field' | 'system'

export type RecordListColumn = DataTableColumn & {
  source: RecordListColumnSource
  cellType: string
  field?: MetadataField
}

export function buildRecordListColumns(fields: MetadataField[], systemFields: MetadataField[] = []): RecordListColumn[] {
  const seen = new Set<string>()
  const systemColumns = recordSystemListColumns(systemFields)
  const columns: RecordListColumn[] = [
    ...systemColumns.filter((column) => column.source === 'name'),
    ...fields.filter((field) => field.listable && !field['write-only']).map((field) => ({
      key: field.name,
      label: field.label || field.name,
      source: 'field' as const,
      cellType: field.studio?.display || 'text',
      sortable: true,
      field,
    })),
    ...systemColumns.filter((column) => column.source !== 'name'),
  ]

  return columns.filter((column) => {
    if (column.key === 'id' || seen.has(column.key)) {
      return false
    }

    seen.add(column.key)
    return true
  })
}
