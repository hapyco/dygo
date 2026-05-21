import { defineStore } from 'pinia'

import type { DataTableRowKey } from '@/design/types'
import { listRecords, RecordApiError, type RecordData } from '@/features/records/records.api'
import { statusForError, storeError, type LoadStatus, type StoreError } from './status'

const defaultPageSize = 20
const pageSizeStorageKey = 'dygo.studio.records.pageSize'
const allowedPageSizes = new Set([20, 100, 500, 2500])

type LoadInitialOptions = {
  pageSize?: number
  force?: boolean
}

export type RecordEntityState = {
  rows: RecordData[]
  total: number
  pageSize: number
  selectedRowKeys: DataTableRowKey[]
  status: LoadStatus
  loadingMore: boolean
  error: StoreError | null
  stale: boolean
}

type RecordsState = {
  pageSize: number
  byEntity: Record<string, RecordEntityState>
  pendingInitialByEntity: Record<string, Promise<RecordEntityState> | undefined>
}

function newRecordEntityState(pageSize: number): RecordEntityState {
  return {
    rows: [],
    total: 0,
    pageSize,
    selectedRowKeys: [],
    status: 'idle',
    loadingMore: false,
    error: null,
    stale: false,
  }
}

function readStoredPageSize(): number {
  if (typeof window === 'undefined') {
    return defaultPageSize
  }

  const value = Number(window.localStorage.getItem(pageSizeStorageKey))
  if (!allowedPageSizes.has(value)) {
    return defaultPageSize
  }

  return value
}

function writeStoredPageSize(pageSize: number) {
  if (typeof window === 'undefined' || !allowedPageSizes.has(pageSize)) {
    return
  }

  window.localStorage.setItem(pageSizeStorageKey, String(pageSize))
}

export const useRecordsStore = defineStore('records', {
  state: (): RecordsState => ({
    pageSize: readStoredPageSize(),
    byEntity: {},
    pendingInitialByEntity: {},
  }),

  getters: {
    entityState: (state) => (entity: string): RecordEntityState => (
      state.byEntity[entity] ?? newRecordEntityState(state.pageSize)
    ),
  },

  actions: {
    ensureEntity(entity: string): RecordEntityState {
      if (!this.byEntity[entity]) {
        this.byEntity[entity] = newRecordEntityState(this.pageSize)
      }

      return this.byEntity[entity]
    },

    async loadInitial(entity: string, options: LoadInitialOptions = {}): Promise<RecordEntityState> {
      if (options.pageSize) {
        this.setGlobalPageSize(options.pageSize)
      }

      const state = this.ensureEntity(entity)

      if (!options.force && !state.stale && (state.status === 'ready' || state.status === 'empty')) {
        return state
      }

      const pending = this.pendingInitialByEntity[entity]
      if (pending && !options.force) {
        return pending
      }

      state.status = 'loading'
      state.error = null
      state.rows = []
      state.total = 0
      state.selectedRowKeys = []

      const request = listRecords(entity, { limit: state.pageSize, offset: 0 })
        .then((result) => {
          state.rows = result.data
          state.total = result.meta.total ?? result.data.length
          state.status = result.data.length === 0 ? 'empty' : 'ready'
          state.error = null
          state.stale = false
          return state
        })
        .catch((error: unknown) => {
          const normalized = storeError(error, 'Studio could not load records.')
          state.rows = []
          state.total = 0
          state.selectedRowKeys = []
          state.error = normalized
          state.status = error instanceof RecordApiError ? statusForError(normalized) : 'error'
          return state
        })
        .finally(() => {
          this.pendingInitialByEntity[entity] = undefined
        })

      this.pendingInitialByEntity[entity] = request
      return request
    },

    async loadMore(entity: string): Promise<RecordEntityState> {
      const state = this.ensureEntity(entity)

      if (state.status === 'loading' || state.loadingMore || state.rows.length >= state.total || state.error) {
        return state
      }

      state.loadingMore = true
      state.error = null

      try {
        const result = await listRecords(entity, { limit: state.pageSize, offset: state.rows.length })
        state.rows = [...state.rows, ...result.data]
        state.total = result.meta.total ?? state.rows.length
        state.status = state.rows.length === 0 ? 'empty' : 'ready'
        state.stale = false
      } catch (error: unknown) {
        const normalized = storeError(error, 'Studio could not load more records.')
        state.error = normalized
        state.status = error instanceof RecordApiError ? statusForError(normalized) : 'error'
      } finally {
        state.loadingMore = false
      }

      return state
    },

    async setPageSize(entity: string, pageSize: number): Promise<RecordEntityState> {
      this.setGlobalPageSize(pageSize)
      return this.loadInitial(entity, { force: true })
    },

    setGlobalPageSize(pageSize: number) {
      if (!allowedPageSizes.has(pageSize)) {
        return
      }

      this.pageSize = pageSize
      writeStoredPageSize(pageSize)

      Object.values(this.byEntity).forEach((state) => {
        if (state.pageSize === pageSize) {
          return
        }

        state.pageSize = pageSize
        state.selectedRowKeys = []
        state.stale = true
      })
    },

    setSelectedRowKeys(entity: string, keys: DataTableRowKey[]) {
      const state = this.ensureEntity(entity)
      state.selectedRowKeys = keys
    },

    resetEntity(entity: string) {
      this.byEntity[entity] = newRecordEntityState(this.pageSize)
      this.pendingInitialByEntity[entity] = undefined
    },
  },
})
