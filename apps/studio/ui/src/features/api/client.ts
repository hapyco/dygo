import type { StudioDialogRequest } from '../dialogs/dialogs.store'
import type { StudioToastRequest } from '../toasts/toasts.store'

export type APIDialogHandler = (dialog: StudioDialogRequest) => void
export type APIToastHandler = (toast: StudioToastRequest) => void

let apiDialogHandler: APIDialogHandler | null = null
let apiToastHandler: APIToastHandler | null = null

export function setAPIDialogHandler(handler: APIDialogHandler | null) {
  apiDialogHandler = handler
}

export function setAPIToastHandler(handler: APIToastHandler | null) {
  apiToastHandler = handler
}

export type ApiErrorBody = {
  code?: string
  message?: string
  details?: Record<string, unknown>
  dialog?: StudioDialogRequest
  toast?: StudioToastRequest
}

export type ApiErrorEnvelope = {
  error?: ApiErrorBody
}

export type DataEnvelope<T> = {
  data: T
  dialog?: StudioDialogRequest
  toast?: StudioToastRequest
}

export type ListEnvelope<T, M = unknown> = {
  data: T
  meta: M
  dialog?: StudioDialogRequest
  toast?: StudioToastRequest
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
    emitAPIToast(payload.error?.toast)
    throw new options.error(payload.error?.code ?? options.fallbackCode, options.message(payload), payload.error?.details)
  }

  const successPayload = payload as TEnvelope & { dialog?: StudioDialogRequest, toast?: StudioToastRequest }
  emitAPIDialog(successPayload.dialog)
  emitAPIToast(successPayload.toast)
  return payload
}

function emitAPIDialog(dialog: StudioDialogRequest | undefined) {
  if (dialog) {
    try {
      apiDialogHandler?.({ ...dialog, source: 'server' })
    } catch {
      // Dialog rendering is best-effort; it must not change API request semantics.
    }
  }
}

function emitAPIToast(toast: StudioToastRequest | undefined) {
  if (toast) {
    try {
      apiToastHandler?.(toast)
    } catch {
      // Toast rendering is best-effort; it must not change API request semantics.
    }
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
