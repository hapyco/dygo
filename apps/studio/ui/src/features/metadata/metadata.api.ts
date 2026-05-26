import { ApiClientError, apiRequest, type ApiErrorEnvelope, type DataEnvelope } from '@/features/api/client'

export type MetadataAppRef = {
  name: string
  label: string
}

export type MetadataEntity = {
  name: string
  key: string
  slug: string | null
  label: string
  description: string
  icon?: string
  'is-single': boolean
  'is-system': boolean
  'is-collection': boolean
  naming?: MetadataNaming
  app: MetadataAppRef
}

export type MetadataNaming = {
  strategy: 'manual' | 'random' | 'format' | 'series'
  label?: string
  length?: number
  pattern?: string
  format?: string
}

export type MetadataField = {
  name: string
  label: string
  type: string
  required: boolean
  unique: boolean
  index: boolean
  stored: boolean
  'write-only': boolean
  listable: boolean
  'name-renderable': boolean
  'value-kind': string
  studio: {
    editor: string
    display: string
  }
  default?: unknown
  check?: unknown
  position: number
  options?: unknown
}

export type MetadataEntityMeta = MetadataEntity & {
  fields: MetadataField[]
  'system-fields': MetadataField[]
  indexes: unknown[]
  constraints: unknown[]
  collections?: Record<string, MetadataEntityMeta>
}

export class MetadataApiError extends ApiClientError {
  constructor(code: string, message: string, details?: Record<string, unknown>) {
    super('MetadataApiError', code, message, details)
  }
}

export async function listEntities(): Promise<MetadataEntity[]> {
  const payload = await apiRequest<DataEnvelope<MetadataEntity[]>, MetadataApiError>('/api/v1/entities', {
    method: 'GET',
  }, metadataRequestOptions())

  return payload.data
}

export async function getEntityMeta(entity: string): Promise<MetadataEntityMeta> {
  const payload = await apiRequest<DataEnvelope<MetadataEntityMeta>, MetadataApiError>(`/api/v1/entities/${encodeURIComponent(entity)}/meta`, {
    method: 'GET',
  }, metadataRequestOptions())

  return payload.data
}

function metadataRequestOptions() {
  return {
    error: MetadataApiError,
    fallbackCode: 'metadata_failed',
    invalidResponseMessage: 'Studio could not read the metadata response.',
    message: metadataErrorMessage,
  }
}

function metadataErrorMessage(payload: ApiErrorEnvelope): string {
  switch (payload.error?.code) {
    case 'unauthenticated':
      return 'Sign in to load Studio metadata.'
    case 'forbidden':
      return 'You do not have permission to read this metadata.'
    case 'not_found':
      return payload.error.message ?? 'Studio could not find this entity.'
    case 'schema_not_ready':
      return 'Studio metadata is not ready yet. Run dygo db migrate, then try again.'
    default:
      return payload.error?.message ?? 'Studio could not load metadata.'
  }
}
