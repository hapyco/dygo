export type MetadataAppRef = {
  name: string
  label: string
}

export type MetadataEntity = {
  name: string
  'route-slug': string
  label: string
  description: string
  icon?: string
  app: MetadataAppRef
}

export type MetadataField = {
  name: string
  label: string
  type: string
  required: boolean
  unique: boolean
  index: boolean
  default?: unknown
  check?: unknown
  position: number
  options?: unknown
}

export type MetadataEntityMeta = MetadataEntity & {
  fields: MetadataField[]
  indexes: unknown[]
  constraints: unknown[]
}

type ApiErrorEnvelope = {
  error?: {
    code?: string
    message?: string
    details?: Record<string, unknown>
  }
}

type DataEnvelope<T> = {
  data: T
}

export class MetadataApiError extends Error {
  readonly code: string
  readonly details?: Record<string, unknown>

  constructor(code: string, message: string, details?: Record<string, unknown>) {
    super(message)
    this.name = 'MetadataApiError'
    this.code = code
    this.details = details
  }
}

export async function listEntities(): Promise<MetadataEntity[]> {
  const response = await fetch('/api/v1/entities', {
    method: 'GET',
    credentials: 'include',
  })

  const payload = await parseJSON<DataEnvelope<MetadataEntity[]> & ApiErrorEnvelope>(response)

  if (!response.ok) {
    throw new MetadataApiError(payload.error?.code ?? 'metadata_failed', metadataErrorMessage(payload), payload.error?.details)
  }

  return payload.data
}

export async function getEntityMeta(entity: string): Promise<MetadataEntityMeta> {
  const response = await fetch(`/api/v1/entities/${encodeURIComponent(entity)}/meta`, {
    method: 'GET',
    credentials: 'include',
  })

  const payload = await parseJSON<DataEnvelope<MetadataEntityMeta> & ApiErrorEnvelope>(response)

  if (!response.ok) {
    throw new MetadataApiError(payload.error?.code ?? 'metadata_failed', metadataErrorMessage(payload), payload.error?.details)
  }

  return payload.data
}

async function parseJSON<T>(response: Response): Promise<T> {
  try {
    return (await response.json()) as T
  } catch {
    throw new MetadataApiError('invalid_response', 'Studio could not read the metadata response.')
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
      return 'Studio metadata is not ready yet. Run dygo migrate, then try again.'
    default:
      return payload.error?.message ?? 'Studio could not load metadata.'
  }
}
