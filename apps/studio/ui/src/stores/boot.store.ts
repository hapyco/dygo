import { defineStore } from 'pinia'

import {
  BootApiError,
  getBoot,
  type StudioBoot,
  type StudioBootDefaults,
} from '@/features/boot/boot.api'
import { statusForError, storeError, type LoadStatus, type StoreError } from './status'

type BootState = {
  boot: StudioBoot | null
  status: LoadStatus
  error: StoreError | null
  loaded: boolean
  pendingBoot: Promise<StudioBoot | null> | null
}

export const useBootStore = defineStore('boot', {
  state: (): BootState => ({
    boot: null,
    status: 'idle',
    error: null,
    loaded: false,
    pendingBoot: null,
  }),

  getters: {
    defaults: (state): StudioBootDefaults | null => state.boot?.defaults ?? null,
  },

  actions: {
    setBoot(boot: StudioBoot) {
      this.boot = boot
      this.loaded = true
      this.status = 'ready'
      this.error = null
      this.pendingBoot = null
    },

    clearBoot() {
      this.boot = null
      this.loaded = false
      this.status = 'idle'
      this.error = null
      this.pendingBoot = null
    },

    async loadBoot(options: { force?: boolean } = {}): Promise<StudioBoot | null> {
      if (this.loaded && this.boot && !options.force) {
        return this.boot
      }

      if (this.pendingBoot && !options.force) {
        return this.pendingBoot
      }

      this.status = 'loading'
      this.error = null

      this.pendingBoot = getBoot()
        .then((boot) => {
          this.setBoot(boot)
          return boot
        })
        .catch((error: unknown) => {
          const normalized = storeError(error, 'Studio could not load boot settings.')
          this.boot = null
          this.loaded = true
          this.error = normalized
          this.status = error instanceof BootApiError ? statusForError(normalized) : 'error'
          return null
        })
        .finally(() => {
          this.pendingBoot = null
        })

      return this.pendingBoot
    },
  },
})
