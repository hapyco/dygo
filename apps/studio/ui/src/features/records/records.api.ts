import type { DataTableSort } from '@/design/types'

export type RecordValue = unknown

export type RecordData = Record<string, RecordValue>

export type RecordListMeta = {
  limit: number
  offset: number
  count: number
  total?: number
}

type ApiErrorEnvelope = {
  error?: {
    code?: string
    message?: string
    details?: Record<string, unknown>
  }
}

type ListEnvelope<T> = {
  data: T
  meta: RecordListMeta
}

export class RecordApiError extends Error {
  readonly code: string
  readonly details?: Record<string, unknown>

  constructor(code: string, message: string, details?: Record<string, unknown>) {
    super(message)
    this.name = 'RecordApiError'
    this.code = code
    this.details = details
  }
}

export type ListRecordsParams = {
  limit: number
  offset: number
  sort?: DataTableSort | null
}

export async function listRecords(entity: string, params: ListRecordsParams): Promise<ListEnvelope<RecordData[]>> {
  const query = new URLSearchParams({
    limit: String(params.limit),
    offset: String(params.offset),
  })

  if (params.sort) {
    query.set('sort', `${params.sort.direction === 'desc' ? '-' : ''}${params.sort.key}`)
  }

  const response = await fetch(`/api/v1/records/${encodeURIComponent(entity)}?${query.toString()}`, {
    method: 'GET',
    credentials: 'include',
  })

  const payload = await parseJSON<ListEnvelope<RecordData[]> & ApiErrorEnvelope>(response)

  if (!response.ok) {
    throw new RecordApiError(payload.error?.code ?? 'records_failed', recordErrorMessage(payload), payload.error?.details)
  }

  return payload
}

async function parseJSON<T>(response: Response): Promise<T> {
  try {
    return (await response.json()) as T
  } catch {
    throw new RecordApiError('invalid_response', 'Studio could not read the records response.')
  }
}

function recordErrorMessage(payload: ApiErrorEnvelope): string {
  switch (payload.error?.code) {
    case 'unauthenticated':
      return 'Sign in to load records.'
    case 'forbidden':
      return 'You do not have permission to read these records.'
    case 'not_found':
      return payload.error.message ?? 'Studio could not find this record list.'
    case 'schema_not_ready':
      return 'Record metadata is not ready yet. Run dygo migrate, then try again.'
    default:
      return payload.error?.message ?? 'Studio could not load records.'
  }
}
