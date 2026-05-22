import { defineStore } from 'pinia'

import type { DataTableRowKey, DataTableSort } from '@/design/types'
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
  sort: DataTableSort | null
  selectedRowKeys: DataTableRowKey[]
  status: LoadStatus
  loadingMore: boolean
  error: StoreError | null
  loadMoreError: StoreError | null
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
    sort: null,
    selectedRowKeys: [],
    status: 'idle',
    loadingMore: false,
    error: null,
    loadMoreError: null,
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

function sortsEqual(left: DataTableSort | null, right: DataTableSort | null): boolean {
  return left?.key === right?.key && left?.direction === right?.direction
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
      state.loadMoreError = null
      state.rows = []
      state.total = 0
      state.selectedRowKeys = []

      const request = listRecords(entity, { limit: state.pageSize, offset: 0, sort: state.sort })
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
          state.loadMoreError = null
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
      state.loadMoreError = null

      try {
        const result = await listRecords(entity, { limit: state.pageSize, offset: state.rows.length, sort: state.sort })
        state.rows = [...state.rows, ...result.data]
        state.total = result.meta.total ?? state.rows.length
        state.status = state.rows.length === 0 ? 'empty' : 'ready'
        state.stale = false
      } catch (error: unknown) {
        const normalized = storeError(error, 'Studio could not load more records.')
        state.loadMoreError = normalized
      } finally {
        state.loadingMore = false
      }

      return state
    },

    async setPageSize(entity: string, pageSize: number): Promise<RecordEntityState> {
      this.setGlobalPageSize(pageSize)
      return this.loadInitial(entity, { force: true })
    },

    async setSort(entity: string, sort: DataTableSort | null): Promise<RecordEntityState> {
      const state = this.ensureEntity(entity)
      if (sortsEqual(state.sort, sort)) {
        return state
      }

      state.sort = sort
      state.selectedRowKeys = []
      state.loadMoreError = null
      state.stale = true
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
