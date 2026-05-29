import type { DataTableSort } from '../../design/types'

type RouteQueryValue = string | null | undefined | (string | null)[]
export type RecordListRouteQuery = Record<string, RouteQueryValue>

export type RecordListFilter = {
  field: string
  operator: string
  value?: string
}

export type ListRecordsParams = {
  limit: number
  offset: number
  sort?: DataTableSort | null
  filters?: RecordListFilter[]
}

export type RecordListRouteState = {
  sort: DataTableSort | null
  filters: RecordListFilter[]
}

export type RecordListRouteFilterOperator = {
  key: string
  arity: 'none' | 'one' | 'range'
}

export type RecordListRouteFilterField = {
  field: string
  operators: readonly RecordListRouteFilterOperator[]
}

export type RecordListRouteSchema = {
  sortableFields: readonly string[]
  filterFields: readonly RecordListRouteFilterField[]
}

export type CanonicalRecordListRouteQuery = {
  state: RecordListRouteState
  query: Record<string, string | string[]>
  changed: boolean
}

export function buildRecordListQuery(params: ListRecordsParams): URLSearchParams {
  const query = new URLSearchParams({
    limit: String(params.limit),
    offset: String(params.offset),
  })

  if (params.sort) {
    query.set('sort', `${params.sort.direction === 'desc' ? '-' : ''}${params.sort.key}`)
  }

  const filters = params.filters ?? []
  filters.forEach((filter) => {
    query.append(`${filter.field}:${filter.operator}`, filter.value ?? '')
  })

  return query
}

export function buildRecordListRouteQuery(state: RecordListRouteState): Record<string, string | string[]> {
  const query: Record<string, string | string[]> = {}

  if (state.sort) {
    query.sort = `${state.sort.direction === 'desc' ? '-' : ''}${state.sort.key}`
  }

  state.filters.forEach((filter) => {
    appendRouteQueryValue(query, recordFilterQueryKey(filter), filter.value ?? '')
  })

  return query
}

export function parseRecordListRouteQuery(query: RecordListRouteQuery): RecordListRouteState {
  const filters: RecordListFilter[] = []

  Object.entries(query).forEach(([key, rawValue]) => {
    if (key === 'limit' || key === 'offset' || key === 'sort') {
      return
    }

    const [field, operator] = parseRecordFilterQueryKey(key)
    if (!field || !operator) {
      return
    }

    routeQueryValues(rawValue).forEach((value) => {
      const filter: RecordListFilter = {
        field,
        operator,
      }
      if (value !== '') {
        filter.value = value
      }
      filters.push(filter)
    })
  })

  return {
    sort: parseRecordListSort(routeQueryValues(query.sort)[0] ?? ''),
    filters,
  }
}

export function canonicalizeRecordListRouteQuery(query: RecordListRouteQuery, schema: RecordListRouteSchema): CanonicalRecordListRouteQuery {
  const state = canonicalizeRecordListRouteState(parseRecordListRouteQuery(query), schema)
  const canonicalQuery = buildRecordListRouteQuery(state)

  return {
    state,
    query: canonicalQuery,
    changed: !recordListRouteQueriesEqual(query, canonicalQuery),
  }
}

export function recordListRouteQueriesEqual(left: RecordListRouteQuery, right: Record<string, string | string[]>): boolean {
  const leftKeys = Object.keys(left).sort()
  const rightKeys = Object.keys(right).sort()
  if (leftKeys.length !== rightKeys.length || leftKeys.some((key, index) => key !== rightKeys[index])) {
    return false
  }

  return leftKeys.every((key) => (
    routeQueryValues(left[key]).sort().join('\u0000') === routeQueryValues(right[key]).sort().join('\u0000')
  ))
}

export function isAllowedRecordPageSize(pageSize: number, pageSizes: readonly number[]): boolean {
  return pageSizes.includes(pageSize)
}

function canonicalizeRecordListRouteState(state: RecordListRouteState, schema: RecordListRouteSchema): RecordListRouteState {
  const sortableFields = new Set(schema.sortableFields)
  const filterFields = new Map(schema.filterFields.map((field) => [field.field, field]))

  return {
    sort: state.sort && sortableFields.has(state.sort.key) ? state.sort : null,
    filters: state.filters.flatMap((filter) => canonicalizeRecordListRouteFilter(filter, filterFields)),
  }
}

function canonicalizeRecordListRouteFilter(
  filter: RecordListFilter,
  fields: Map<string, RecordListRouteFilterField>,
): RecordListFilter[] {
  const field = fields.get(filter.field)
  const operator = field?.operators.find((candidate) => candidate.key === filter.operator)
  if (!field || !operator) {
    return []
  }

  if (operator.arity === 'none') {
    return [{ field: filter.field, operator: filter.operator }]
  }

  const value = filter.value?.trim() ?? ''
  if (value === '') {
    return []
  }

  return [{ field: filter.field, operator: filter.operator, value }]
}

function recordFilterQueryKey(filter: RecordListFilter): string {
  return `${filter.field}:${filter.operator}`
}

function parseRecordFilterQueryKey(key: string): [string, string] {
  const [field, operator, extra] = key.split(':')
  if (!field || !operator || extra !== undefined) {
    return ['', '']
  }

  return [field, operator]
}

function parseRecordListSort(value: string): DataTableSort | null {
  const term = value.split(',')[0]?.trim() ?? ''
  if (term === '' || term === '-') {
    return null
  }

  if (term.startsWith('-')) {
    const key = term.slice(1).trim()
    return key ? { key, direction: 'desc' } : null
  }

  return { key: term, direction: 'asc' }
}

function routeQueryValues(value: RouteQueryValue): string[] {
  if (Array.isArray(value)) {
    return value.map((entry) => entry ?? '')
  }

  if (value === undefined) {
    return []
  }

  if (value === null) {
    return ['']
  }

  return [value]
}

function appendRouteQueryValue(query: Record<string, string | string[]>, key: string, value: string) {
  const current = query[key]
  if (current === undefined) {
    query[key] = value
    return
  }

  if (Array.isArray(current)) {
    current.push(value)
    return
  }

  query[key] = [current, value]
}
