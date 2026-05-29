import type { DataTableSort } from '@/design/types'
import type { RecordListFilter } from './query'

export function recordListBaseQueryKey(entity: string) {
  return ['records', 'list', entity] as const
}

export function recordListQueryKey(
  entity: string,
  params: {
    pageSize: number
    sort: DataTableSort | null
    filters: RecordListFilter[]
  },
) {
  return [
    ...recordListBaseQueryKey(entity),
    params.pageSize,
    params.sort ? `${params.sort.direction}:${params.sort.key}` : '',
    params.filters.map((filter) => `${filter.field}:${filter.operator}:${filter.value ?? ''}`),
  ] as const
}
