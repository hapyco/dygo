import type { DataTableSort } from '../../design/types'

export type ListRecordsParams = {
  limit: number
  offset: number
  sort?: DataTableSort | null
  filters?: Record<string, string>
}

export function buildRecordListQuery(params: ListRecordsParams): URLSearchParams {
  const query = new URLSearchParams({
    limit: String(params.limit),
    offset: String(params.offset),
  })

  if (params.sort) {
    query.set('sort', `${params.sort.direction === 'desc' ? '-' : ''}${params.sort.key}`)
  }

  Object.entries(params.filters ?? {}).forEach(([key, value]) => {
    query.set(key, value)
  })

  return query
}

export function isAllowedRecordPageSize(pageSize: number, pageSizes: readonly number[]): boolean {
  return pageSizes.includes(pageSize)
}
