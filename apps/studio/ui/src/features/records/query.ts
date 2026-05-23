import type { DataTableSort } from '../../design/types'

export const recordListDefaultLimit = 50
export const recordListMaxLimit = 2500
export const recordListPageSizes = [20, 100, 500, recordListMaxLimit] as const
export const recordListDefaultPageSize = 20

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

export function isAllowedRecordPageSize(pageSize: number): boolean {
  return recordListPageSizes.includes(pageSize as (typeof recordListPageSizes)[number])
}
