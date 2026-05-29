import { ApiClientError, apiRequest, type ApiErrorEnvelope, type DataEnvelope, type ListEnvelope } from '@/features/api/client'
import { buildRecordListQuery, type ListRecordsParams } from './query'

export type RecordValue = unknown

export type RecordData = Record<string, RecordValue>

export type RecordListMeta = {
  limit: number
  offset: number
  count: number
  total?: number
}

type ListRecordsOptions = {
  signal?: AbortSignal
}

type ReadRecordOptions = {
  signal?: AbortSignal
}

export class RecordApiError extends ApiClientError {
  constructor(code: string, message: string, details?: Record<string, unknown>) {
    super('RecordApiError', code, message, details)
  }
}

export async function listRecords(entity: string, params: ListRecordsParams, options: ListRecordsOptions = {}): Promise<ListEnvelope<RecordData[], RecordListMeta>> {
  const query = buildRecordListQuery(params)

  return apiRequest<ListEnvelope<RecordData[], RecordListMeta>, RecordApiError>(`/api/v1/records/${encodeURIComponent(entity)}?${query.toString()}`, {
    method: 'GET',
    signal: options.signal,
  }, recordRequestOptions('records_failed'))
}

export async function getRecordByName(entity: string, recordName: string, options: ReadRecordOptions = {}): Promise<RecordData> {
  const payload = await apiRequest<DataEnvelope<RecordData>, RecordApiError>(`/api/v1/records/${encodeURIComponent(entity)}/name/${encodeURIComponent(recordName)}`, {
    method: 'GET',
    signal: options.signal,
  }, recordRequestOptions('record_lookup_failed'))

  return payload.data
}

export async function getSingleRecord(entity: string, options: ReadRecordOptions = {}): Promise<RecordData> {
  const payload = await apiRequest<DataEnvelope<RecordData>, RecordApiError>(`/api/v1/records/${encodeURIComponent(entity)}/single`, {
    method: 'GET',
    signal: options.signal,
  }, recordRequestOptions('single_record_lookup_failed'))

  return payload.data
}

export async function createRecord(entity: string, data: RecordData): Promise<RecordData> {
  const payload = await apiRequest<DataEnvelope<RecordData>, RecordApiError>(`/api/v1/records/${encodeURIComponent(entity)}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ data }),
  }, recordRequestOptions('record_create_failed'))

  return payload.data
}

export async function updateRecord(entity: string, id: string | number, data: RecordData): Promise<RecordData> {
  const payload = await apiRequest<DataEnvelope<RecordData>, RecordApiError>(`/api/v1/records/${encodeURIComponent(entity)}/${encodeURIComponent(String(id))}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ data }),
  }, recordRequestOptions('record_update_failed'))

  return payload.data
}

export async function deleteRecord(entity: string, id: string | number): Promise<void> {
  await apiRequest<DataEnvelope<{ deleted: boolean }>, RecordApiError>(`/api/v1/records/${encodeURIComponent(entity)}/${encodeURIComponent(String(id))}`, {
    method: 'DELETE',
  }, recordRequestOptions('record_delete_failed'))
}

export async function updateSingleRecord(entity: string, data: RecordData): Promise<RecordData> {
  const payload = await apiRequest<DataEnvelope<RecordData>, RecordApiError>(`/api/v1/records/${encodeURIComponent(entity)}/single`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ data }),
  }, recordRequestOptions('single_record_update_failed'))

  return payload.data
}

function recordRequestOptions(fallbackCode: string) {
  return {
    error: RecordApiError,
    fallbackCode,
    invalidResponseMessage: 'Studio could not read the records response.',
    message: recordErrorMessage,
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
      return 'Record metadata is not ready yet. Run dygo db migrate, then try again.'
    default:
      return payload.error?.message ?? 'Studio could not load records.'
  }
}
