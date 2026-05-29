import { ApiClientError, apiRequest, type ApiErrorEnvelope, type DataEnvelope } from '../api/client.ts'

export type RecordListPolicy = {
  'default-limit': number
  'max-limit': number
  'page-sizes': number[]
}

export type PlatformConfig = {
  'record-list': RecordListPolicy
}

export class PlatformApiError extends ApiClientError {
  constructor(code: string, message: string, details?: Record<string, unknown>) {
    super('PlatformApiError', code, message, details)
  }
}

type PlatformRequestOptions = {
  signal?: AbortSignal
}

export async function getPlatformConfig(options: PlatformRequestOptions = {}): Promise<PlatformConfig> {
  const payload = await apiRequest<DataEnvelope<PlatformConfig>, PlatformApiError>('/api/v1/platform', {
    method: 'GET',
    signal: options.signal,
  }, {
    error: PlatformApiError,
    fallbackCode: 'platform_failed',
    invalidResponseMessage: 'Studio could not read the platform response.',
    message: platformErrorMessage,
  })

  return payload.data
}

function platformErrorMessage(payload: ApiErrorEnvelope): string {
  switch (payload.error?.code) {
    case 'unauthenticated':
      return 'Sign in to load Studio platform settings.'
    case 'forbidden':
      return 'You do not have permission to read platform settings.'
    default:
      return payload.error?.message ?? 'Studio could not load platform settings.'
  }
}
