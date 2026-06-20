import type { StudioDialogRequest } from '../dialogs/dialogs.store'

export type APIDialogHandler = (dialog: StudioDialogRequest) => void

let apiDialogHandler: APIDialogHandler | null = null

export function setAPIDialogHandler(handler: APIDialogHandler | null) {
  apiDialogHandler = handler
}

export type ApiErrorBody = {
  code?: string
  message?: string
  details?: Record<string, unknown>
  dialog?: StudioDialogRequest
}

export type ApiErrorEnvelope = {
  error?: ApiErrorBody
}

export type DataEnvelope<T> = {
  data: T
  dialog?: StudioDialogRequest
}

export type ListEnvelope<T, M = unknown> = {
  data: T
  meta: M
  dialog?: StudioDialogRequest
}

export class ApiClientError extends Error {
  readonly code: string
  readonly details?: Record<string, unknown>

  constructor(name: string, code: string, message: string, details?: Record<string, unknown>) {
    super(message)
    this.name = name
    this.code = code
    this.details = details
  }
}

export type ApiErrorClass<T extends ApiClientError> = new (
  code: string,
  message: string,
  details?: Record<string, unknown>,
) => T

export type ApiRequestOptions<TError extends ApiClientError> = {
  error: ApiErrorClass<TError>
  fallbackCode: string
  invalidResponseMessage: string
  message: (payload: ApiErrorEnvelope) => string
}

export async function apiRequest<TEnvelope, TError extends ApiClientError>(
  input: RequestInfo | URL,
  init: RequestInit,
  options: ApiRequestOptions<TError>,
): Promise<TEnvelope> {
  const response = await fetch(input, {
    credentials: 'include',
    ...init,
  })
  const payload = await parseAPIJSON<TEnvelope & ApiErrorEnvelope>(response, options.error, options.invalidResponseMessage)

  if (!response.ok) {
    emitAPIDialog(payload.error?.dialog)
    throw new options.error(payload.error?.code ?? options.fallbackCode, options.message(payload), payload.error?.details)
  }

  emitAPIDialog((payload as TEnvelope & { dialog?: StudioDialogRequest }).dialog)
  return payload
}

function emitAPIDialog(dialog: StudioDialogRequest | undefined) {
  if (dialog) {
    apiDialogHandler?.({ ...dialog, source: 'server' })
  }
}

async function parseAPIJSON<TEnvelope, TError extends ApiClientError = ApiClientError>(
  response: Response,
  ErrorClass: ApiErrorClass<TError>,
  invalidResponseMessage: string,
): Promise<TEnvelope> {
  try {
    return (await response.json()) as TEnvelope
  } catch {
    throw new ErrorClass('invalid_response', invalidResponseMessage)
  }
}
