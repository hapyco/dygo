export type LoadStatus = 'idle' | 'loading' | 'ready' | 'empty' | 'error' | 'forbidden' | 'unauthenticated'

export type StoreError = {
  code: string
  message: string
  details?: Record<string, unknown>
}

type ApiLikeError = Error & {
  code?: string
  details?: Record<string, unknown>
}

export function storeError(error: unknown, fallbackMessage: string): StoreError {
  if (error instanceof Error) {
    const apiError = error as ApiLikeError

    return {
      code: apiError.code ?? 'error',
      message: error.message || fallbackMessage,
      details: apiError.details,
    }
  }

  return {
    code: 'error',
    message: fallbackMessage,
  }
}

export function statusForError(error: StoreError): LoadStatus {
  if (error.code === 'unauthenticated') {
    return 'unauthenticated'
  }

  if (error.code === 'forbidden') {
    return 'forbidden'
  }

  return 'error'
}
