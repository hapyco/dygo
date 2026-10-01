import { ApiClientError, apiRequest, type ApiErrorEnvelope, type DataEnvelope } from '@/features/api/client'

export type MetadataAppRef = {
  name: string
  label: string
}

export type MetadataEntity = {
  tree?: { 'parent-field': string; 'label-field'?: string }
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

export type MetadataFilterOperator = {
  key: string
  label: string
  arity: 'none' | 'one' | 'range'
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
  filter: {
    operators: MetadataFilterOperator[]
  }
  default?: unknown
  check?: unknown
  fetch?: {
    from: string
  }
  position: number
  options?: unknown
}

export type MetadataLinkFilter = {
  field: string
  operator: string
  value?: unknown
}

export type MetadataLinkOptions = {
  app?: string
  entity: string
  displayField?: string
  filters: MetadataLinkFilter[]
}

export type MetadataFormItem = {
  kind: 'field' | 'column' | 'section'
  name?: string
  label?: string
  description?: string
}

export type MetadataFormTab = {
  key: string
  label: string
  icon?: string
  items: MetadataFormItem[]
}

export type MetadataFormLayout = {
  tabs: MetadataFormTab[]
}

export type MetadataEntityMeta = MetadataEntity & {
  form?: MetadataFormLayout
  fields: MetadataField[]
  'system-fields': MetadataField[]
  indexes: unknown[]
  constraints: unknown[]
  collections?: Record<string, MetadataEntityMeta>
  actions?: EntityActionDefinition[]
}

export type EntityActionDefinition = {
  name: string
  label: string
  selection: 'record' | 'selection' | 'collection'
  confirm?: string
  danger?: boolean
}

type MetadataRequestOptions = {
  signal?: AbortSignal
}

export class MetadataApiError extends ApiClientError {
  constructor(code: string, message: string, details?: Record<string, unknown>) {
    super('MetadataApiError', code, message, details)
  }
}

export function linkOptions(field: MetadataField): MetadataLinkOptions | null {
  if (field.type !== 'link' || !field.options || typeof field.options !== 'object') {
    return null
  }

  const options = field.options as Record<string, unknown>
  if (typeof options.entity !== 'string' || options.entity.trim() === '') {
    return null
  }

  const filters = Array.isArray(options.filters)
    ? options.filters.flatMap((filter): MetadataLinkFilter[] => {
      if (!filter || typeof filter !== 'object') {
        return []
      }
      const value = filter as Record<string, unknown>
      if (typeof value.field !== 'string') {
        return []
      }
      const filterValue = typeof value.from === 'string' ? `$${value.from}` : value.value
      return [{ field: value.field, operator: typeof value.operator === 'string' ? value.operator : 'eq', value: filterValue }]
    })
    : []

  return {
    app: typeof options.app === 'string' ? options.app : undefined,
    entity: options.entity,
    displayField: typeof options['display-field'] === 'string' ? options['display-field'] : undefined,
    filters,
  }
}

export async function listEntities(options: MetadataRequestOptions = {}): Promise<MetadataEntity[]> {
  const payload = await apiRequest<DataEnvelope<MetadataEntity[]>, MetadataApiError>('/api/v1/entities', {
    method: 'GET',
    signal: options.signal,
  }, metadataRequestOptions())

  return payload.data
}

export async function getEntityMeta(entity: string, options: MetadataRequestOptions = {}): Promise<MetadataEntityMeta> {
  const payload = await apiRequest<DataEnvelope<MetadataEntityMeta>, MetadataApiError>(`/api/v1/entities/${encodeURIComponent(entity)}/meta`, {
    method: 'GET',
    signal: options.signal,
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
