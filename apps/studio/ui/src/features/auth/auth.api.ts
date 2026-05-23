import { ApiClientError, apiRequest, type ApiErrorEnvelope, type DataEnvelope } from '@/features/api/client'

export type LoginInput = {
  email: string
  password: string
  remember: boolean
}

export type CurrentUser = {
  id: number
  email: string
  'full-name': string
  enabled: boolean
  administrator: boolean
}

export class AuthApiError extends ApiClientError {
  constructor(code: string, message: string, details?: Record<string, unknown>) {
    super('AuthApiError', code, message, details)
  }
}

export async function login(input: LoginInput): Promise<CurrentUser> {
  const payload = await apiRequest<DataEnvelope<CurrentUser>, AuthApiError>('/api/v1/auth/login', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      data: {
        email: input.email,
        password: input.password,
        remember: input.remember,
      },
    }),
  }, {
    error: AuthApiError,
    fallbackCode: 'login_failed',
    invalidResponseMessage: 'Studio could not read the server response.',
    message: loginErrorMessage,
  })

  return payload.data
}

export async function getCurrentUser(): Promise<CurrentUser> {
  const payload = await apiRequest<DataEnvelope<CurrentUser>, AuthApiError>('/api/v1/auth/me', {
    method: 'GET',
  }, {
    error: AuthApiError,
    fallbackCode: 'unauthenticated',
    invalidResponseMessage: 'Studio could not read the server response.',
    message: currentUserErrorMessage,
  })

  return payload.data
}

function currentUserErrorMessage(payload: ApiErrorEnvelope): string {
  switch (payload.error?.code) {
    case 'unauthenticated':
      return 'Sign in to open Studio.'
    case 'schema_not_ready':
      return 'Studio is not ready yet. Run dygo migrate, then try again.'
    default:
      return 'Studio could not read the current session.'
  }
}

function loginErrorMessage(payload: ApiErrorEnvelope): string {
  switch (payload.error?.code) {
    case 'invalid_credentials':
      return 'Email or password is incorrect.'
    case 'schema_not_ready':
      return 'Studio is not ready yet. Run dygo migrate, then try again.'
    case 'invalid_request':
      return payload.error.message ?? 'Enter a valid email and password.'
    default:
      return 'Sign in failed. Check the server and try again.'
  }
}
