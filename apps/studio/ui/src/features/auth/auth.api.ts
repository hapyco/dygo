export type LoginInput = {
  identifier: string
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

type ApiErrorEnvelope = {
  error?: {
    code?: string
    message?: string
    details?: Record<string, unknown>
  }
}

type LoginEnvelope = {
  data: CurrentUser
}

type CurrentUserEnvelope = {
  data: CurrentUser
}

export class AuthApiError extends Error {
  readonly code: string
  readonly details?: Record<string, unknown>

  constructor(code: string, message: string, details?: Record<string, unknown>) {
    super(message)
    this.name = 'AuthApiError'
    this.code = code
    this.details = details
  }
}

export async function login(input: LoginInput): Promise<CurrentUser> {
  const response = await fetch('/api/v1/auth/login', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    credentials: 'include',
    body: JSON.stringify({
      data: {
        identifier: input.identifier,
        password: input.password,
        remember: input.remember,
      },
    }),
  })

  const payload = await parseJSON<LoginEnvelope & ApiErrorEnvelope>(response)

  if (!response.ok) {
    throw new AuthApiError(payload.error?.code ?? 'login_failed', loginErrorMessage(payload), payload.error?.details)
  }

  return payload.data
}

export async function getCurrentUser(): Promise<CurrentUser> {
  const response = await fetch('/api/v1/auth/me', {
    method: 'GET',
    credentials: 'include',
  })

  const payload = await parseJSON<CurrentUserEnvelope & ApiErrorEnvelope>(response)

  if (!response.ok) {
    throw new AuthApiError(payload.error?.code ?? 'unauthenticated', currentUserErrorMessage(payload), payload.error?.details)
  }

  return payload.data
}

async function parseJSON<T>(response: Response): Promise<T> {
  try {
    return (await response.json()) as T
  } catch (error) {
    throw new AuthApiError('invalid_response', 'Studio could not read the server response.')
  }
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
      return 'Email, username, or password is incorrect.'
    case 'schema_not_ready':
      return 'Studio is not ready yet. Run dygo migrate, then try again.'
    case 'invalid_request':
      return payload.error.message ?? 'Enter a valid email or username and password.'
    default:
      return 'Sign in failed. Check the server and try again.'
  }
}
