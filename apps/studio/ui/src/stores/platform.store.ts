import { defineStore } from 'pinia'

import {
  getPlatformConfig,
  PlatformApiError,
  type PlatformConfig,
  type RecordListPolicy,
} from '@/features/platform/platform.api'
import { statusForError, storeError, type LoadStatus, type StoreError } from './status'

type PlatformState = {
  config: PlatformConfig | null
  status: LoadStatus
  error: StoreError | null
  pendingConfig: Promise<PlatformConfig | null> | null
}

export const usePlatformStore = defineStore('platform', {
  state: (): PlatformState => ({
    config: null,
    status: 'idle',
    error: null,
    pendingConfig: null,
  }),

  getters: {
    recordListPolicy: (state): RecordListPolicy | null => state.config?.['record-list'] ?? null,
  },

  actions: {
    async loadPlatform(options: { force?: boolean } = {}): Promise<PlatformConfig | null> {
      if (this.config && !options.force) {
        return this.config
      }

      if (this.pendingConfig && !options.force) {
        return this.pendingConfig
      }

      this.status = 'loading'
      this.error = null

      this.pendingConfig = getPlatformConfig()
        .then((config) => {
          this.config = config
          this.status = 'ready'
          this.error = null
          return config
        })
        .catch((error: unknown) => {
          const normalized = storeError(error, 'Studio could not load platform settings.')
          this.error = normalized
          this.status = error instanceof PlatformApiError ? statusForError(normalized) : 'error'
          return null
        })
        .finally(() => {
          this.pendingConfig = null
        })

      return this.pendingConfig
    },
  },
})
