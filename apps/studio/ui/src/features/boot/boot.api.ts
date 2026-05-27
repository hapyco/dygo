import { ApiClientError, apiRequest, type ApiErrorEnvelope, type DataEnvelope } from '@/features/api/client'
import type { CurrentUser } from '@/features/auth/auth.api'

export type StudioBootDefaults = {
  home: string
}

export type StudioBoot = {
  user: CurrentUser
  defaults: StudioBootDefaults
}

export class BootApiError extends ApiClientError {
  constructor(code: string, message: string, details?: Record<string, unknown>) {
    super('BootApiError', code, message, details)
  }
}

export async function getBoot(): Promise<StudioBoot> {
  const payload = await apiRequest<DataEnvelope<StudioBoot>, BootApiError>('/api/v1/boot', {
    method: 'GET',
  }, {
    error: BootApiError,
    fallbackCode: 'boot_failed',
    invalidResponseMessage: 'Studio could not read the boot response.',
    message: bootErrorMessage,
  })

  return payload.data
}

function bootErrorMessage(payload: ApiErrorEnvelope): string {
  switch (payload.error?.code) {
    case 'unauthenticated':
      return 'Sign in to load Studio boot settings.'
    case 'schema_not_ready':
      return 'Studio is not ready yet. Run dygo db migrate, then try again.'
    default:
      return payload.error?.message ?? 'Studio could not load boot settings.'
  }
}
