import { defineStore } from 'pinia'

import { AuthApiError, getCurrentUser, logout as logoutRequest, type CurrentUser } from '@/features/auth/auth.api'
import { statusForError, storeError, type LoadStatus, type StoreError } from './status'

type LoadCurrentUserOptions = {
  force?: boolean
}

type AuthState = {
  currentUser: CurrentUser | null
  status: LoadStatus
  error: StoreError | null
  loaded: boolean
  pendingUser: Promise<CurrentUser | null> | null
}

export const useAuthStore = defineStore('auth', {
  state: (): AuthState => ({
    currentUser: null,
    status: 'idle',
    error: null,
    loaded: false,
    pendingUser: null,
  }),

  actions: {
    setCurrentUser(user: CurrentUser | null) {
      this.currentUser = user
      this.loaded = true
      this.error = null
      this.status = user ? 'ready' : 'unauthenticated'
      this.pendingUser = null
    },

    clearSession() {
      this.setCurrentUser(null)
    },

    async logout(): Promise<void> {
      try {
        await logoutRequest()
      } finally {
        this.clearSession()
      }
    },

    async loadCurrentUser(options: LoadCurrentUserOptions = {}): Promise<CurrentUser | null> {
      if (this.loaded && !options.force) {
        return this.currentUser
      }

      if (this.pendingUser && !options.force) {
        return this.pendingUser
      }

      this.status = 'loading'
      this.error = null

      this.pendingUser = getCurrentUser()
        .then((user) => {
          this.setCurrentUser(user)
          return user
        })
        .catch((error: unknown) => {
          const normalized = storeError(error, 'Studio could not read the current session.')
          this.currentUser = null
          this.loaded = true
          this.error = normalized
          this.status = error instanceof AuthApiError ? statusForError(normalized) : 'error'
          return null
        })
        .finally(() => {
          this.pendingUser = null
        })

      return this.pendingUser
    },
  },
})
