import { defineStore } from 'pinia'

import type { DataTableRowKey, DataTableSort } from '@/design/types'
import {
  createRecord as createRecordRequest,
  deleteRecord as deleteRecordRequest,
  getRecordByName,
  getSingleRecord,
  listRecords,
  RecordApiError,
  updateRecord as updateRecordRequest,
  updateSingleRecord as updateSingleRecordRequest,
  type RecordData,
} from '@/features/records/records.api'
import { isAllowedRecordPageSize, type ListRecordsParams, type RecordListFilter, type RecordListRouteState } from '@/features/records/query'
import type { RecordListPolicy } from '@/features/platform/platform.api'
import { usePlatformStore } from './platform.store'
import { statusForError, storeError, type LoadStatus, type StoreError } from './status'

const pageSizeStorageKey = 'dygo.studio.records.pageSize'
export const singleRecordKey = '__single__'

type LoadInitialOptions = {
  pageSize?: number
  force?: boolean
}

type LoadOptions = {
  force?: boolean
}

type ActiveRecordListRequest = {
  id: number
  controller: AbortController
}

export type RecordEntityState = {
  rows: RecordData[]
  total: number
  pageSize: number
  sort: DataTableSort | null
  filters: RecordListFilter[]
  selectedRowKeys: DataTableRowKey[]
  status: LoadStatus
  loadingMore: boolean
  error: StoreError | null
  loadMoreError: StoreError | null
  stale: boolean
}

export type RecordFormState = {
  record: RecordData | null
  status: LoadStatus
  saving: boolean
  error: StoreError | null
  saveError: StoreError | null
}

type RecordsState = {
  pageSize: number
  byEntity: Record<string, RecordEntityState>
  pendingInitialByEntity: Record<string, Promise<RecordEntityState> | undefined>
  byRecord: Record<string, RecordFormState>
  pendingRecordByKey: Record<string, Promise<RecordFormState> | undefined>
}

const activeRecordListRequests = new Map<string, ActiveRecordListRequest>()
let nextRecordListRequestID = 1

function newRecordEntityState(pageSize: number): RecordEntityState {
  return {
    rows: [],
    total: 0,
    pageSize,
    sort: null,
    filters: [],
    selectedRowKeys: [],
    status: 'idle',
    loadingMore: false,
    error: null,
    loadMoreError: null,
    stale: false,
  }
}

function newRecordFormState(): RecordFormState {
  return {
    record: null,
    status: 'idle',
    saving: false,
    error: null,
    saveError: null,
  }
}

function recordStateKey(entity: string, recordName: string): string {
  return JSON.stringify([entity, recordName])
}

function defaultPageSize(policy: RecordListPolicy): number {
  return policy['page-sizes'][0] ?? policy['default-limit']
}

function readStoredPageSize(policy: RecordListPolicy): number {
  if (typeof window === 'undefined') {
    return defaultPageSize(policy)
  }

  const value = Number(window.localStorage.getItem(pageSizeStorageKey))
  if (!isAllowedRecordPageSize(value, policy['page-sizes'])) {
    return defaultPageSize(policy)
  }

  return value
}

function writeStoredPageSize(pageSize: number, policy: RecordListPolicy) {
  if (typeof window === 'undefined' || !isAllowedRecordPageSize(pageSize, policy['page-sizes'])) {
    return
  }

  window.localStorage.setItem(pageSizeStorageKey, String(pageSize))
}

function missingRecordListPolicyError(platformStore: ReturnType<typeof usePlatformStore>): Error {
  const error = new Error(platformStore.error?.message ?? 'Studio could not load record list settings.')
  const apiError = error as Error & { code?: string; details?: Record<string, unknown> }
  apiError.code = platformStore.error?.code ?? 'platform_failed'
  apiError.details = platformStore.error?.details
  return error
}

function sortsEqual(left: DataTableSort | null, right: DataTableSort | null): boolean {
  return left?.key === right?.key && left?.direction === right?.direction
}

function filtersEqual(left: RecordListFilter[], right: RecordListFilter[]): boolean {
  if (left.length !== right.length) {
    return false
  }

  return left.every((filter, index) => {
    const other = right[index]
    return filter.field === other.field && filter.operator === other.operator && filter.value === other.value
  })
}

function recordListParams(state: RecordEntityState, offset: number): ListRecordsParams {
  return {
    limit: state.pageSize,
    offset,
    sort: state.sort ? { ...state.sort } : null,
    filters: state.filters.map((filter) => ({ ...filter })),
  }
}

// TODO: Extract this cancellation/token pattern if another Studio store needs race-safe API requests.
function beginRecordListRequest(entity: string): ActiveRecordListRequest {
  activeRecordListRequests.get(entity)?.controller.abort()

  const request = {
    id: nextRecordListRequestID,
    controller: new AbortController(),
  }
  nextRecordListRequestID += 1
  activeRecordListRequests.set(entity, request)
  return request
}

function isCurrentRecordListRequest(entity: string, request: ActiveRecordListRequest): boolean {
  return activeRecordListRequests.get(entity)?.id === request.id
}

function finishRecordListRequest(entity: string, request: ActiveRecordListRequest) {
  if (isCurrentRecordListRequest(entity, request)) {
    activeRecordListRequests.delete(entity)
  }
}

function cancelRecordListRequest(entity: string) {
  const request = activeRecordListRequests.get(entity)
  if (!request) {
    return
  }

  activeRecordListRequests.delete(entity)
  request.controller.abort()
}

function isAbortError(error: unknown): boolean {
  return typeof error === 'object' && error !== null && 'name' in error && error.name === 'AbortError'
}

export const useRecordsStore = defineStore('records', {
  state: (): RecordsState => ({
    pageSize: 0,
    byEntity: {},
    pendingInitialByEntity: {},
    byRecord: {},
    pendingRecordByKey: {},
  }),

  getters: {
    entityState: (state) => (entity: string): RecordEntityState => (
      state.byEntity[entity] ?? newRecordEntityState(state.pageSize)
    ),

    recordState: (state) => (entity: string, recordName: string): RecordFormState => (
      state.byRecord[recordStateKey(entity, recordName)] ?? newRecordFormState()
    ),
  },

  actions: {
    ensureEntity(entity: string): RecordEntityState {
      if (!this.byEntity[entity]) {
        this.byEntity[entity] = newRecordEntityState(this.pageSize)
      }

      return this.byEntity[entity]
    },

    ensureRecord(entity: string, recordName: string): RecordFormState {
      const key = recordStateKey(entity, recordName)
      if (!this.byRecord[key]) {
        this.byRecord[key] = newRecordFormState()
      }

      return this.byRecord[key]
    },

    async ensureRecordListPolicy(): Promise<RecordListPolicy> {
      const platformStore = usePlatformStore()
      await platformStore.loadPlatform()
      const policy = platformStore.recordListPolicy
      if (!policy) {
        throw missingRecordListPolicyError(platformStore)
      }
      const nextPageSize = readStoredPageSize(policy)

      if (this.pageSize !== nextPageSize) {
        this.pageSize = nextPageSize
        Object.entries(this.byEntity).forEach(([entity, state]) => {
          if (state.pageSize === nextPageSize) {
            return
          }

          cancelRecordListRequest(entity)
          this.pendingInitialByEntity[entity] = undefined
          state.pageSize = nextPageSize
          state.loadingMore = false
          state.selectedRowKeys = []
          state.stale = true
        })
      }

      return policy
    },

    async loadInitial(entity: string, options: LoadInitialOptions = {}): Promise<RecordEntityState> {
      const state = this.ensureEntity(entity)

      try {
        await this.ensureRecordListPolicy()
      } catch (error: unknown) {
        const normalized = storeError(error, 'Studio could not load record list settings.')
        state.rows = []
        state.total = 0
        state.selectedRowKeys = []
        state.error = normalized
        state.loadMoreError = null
        state.status = statusForError(normalized)
        return state
      }

      if (options.pageSize) {
        this.setGlobalPageSize(options.pageSize)
      }

      if (!options.force && !state.stale && (state.status === 'ready' || state.status === 'empty')) {
        return state
      }

      const pending = this.pendingInitialByEntity[entity]
      if (pending && !options.force) {
        return pending
      }

      const activeRequest = beginRecordListRequest(entity)
      const params = recordListParams(state, 0)

      state.status = 'loading'
      state.loadingMore = false
      state.error = null
      state.loadMoreError = null
      state.rows = []
      state.total = 0
      state.selectedRowKeys = []

      const request = listRecords(entity, params, { signal: activeRequest.controller.signal })
        .then((result) => {
          if (!isCurrentRecordListRequest(entity, activeRequest)) {
            return state
          }

          state.rows = result.data
          state.total = result.meta.total ?? result.data.length
          state.status = result.data.length === 0 ? 'empty' : 'ready'
          state.error = null
          state.stale = false
          return state
        })
        .catch((error: unknown) => {
          if (!isCurrentRecordListRequest(entity, activeRequest) || isAbortError(error)) {
            return state
          }

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
          if (isCurrentRecordListRequest(entity, activeRequest)) {
            this.pendingInitialByEntity[entity] = undefined
            finishRecordListRequest(entity, activeRequest)
          }
        })

      this.pendingInitialByEntity[entity] = request
      return request
    },

    async loadRecordByName(entity: string, recordName: string, options: LoadOptions = {}): Promise<RecordFormState> {
      const state = this.ensureRecord(entity, recordName)

      if (!options.force && state.status === 'ready') {
        return state
      }

      const key = recordStateKey(entity, recordName)
      const pending = this.pendingRecordByKey[key]
      if (pending && !options.force) {
        return pending
      }

      state.status = 'loading'
      state.error = null
      state.saveError = null
      state.record = null

      const request = getRecordByName(entity, recordName)
        .then((record) => {
          state.record = record
          state.status = 'ready'
          state.error = null
          return state
        })
        .catch((error: unknown) => {
          const normalized = storeError(error, 'Studio could not load this record.')
          state.record = null
          state.error = normalized
          state.saveError = null
          state.status = error instanceof RecordApiError ? statusForError(normalized) : 'error'
          return state
        })
        .finally(() => {
          this.pendingRecordByKey[key] = undefined
        })

      this.pendingRecordByKey[key] = request
      return request
    },

    async loadSingleRecord(entity: string, options: LoadOptions = {}): Promise<RecordFormState> {
      const state = this.ensureRecord(entity, singleRecordKey)

      if (!options.force && state.status === 'ready') {
        return state
      }

      const key = recordStateKey(entity, singleRecordKey)
      const pending = this.pendingRecordByKey[key]
      if (pending && !options.force) {
        return pending
      }

      state.status = 'loading'
      state.error = null
      state.saveError = null
      state.record = null

      const request = getSingleRecord(entity)
        .then((record) => {
          state.record = record
          state.status = 'ready'
          state.error = null
          return state
        })
        .catch((error: unknown) => {
          const normalized = storeError(error, 'Studio could not load these settings.')
          state.record = null
          state.error = normalized
          state.saveError = null
          state.status = error instanceof RecordApiError ? statusForError(normalized) : 'error'
          return state
        })
        .finally(() => {
          this.pendingRecordByKey[key] = undefined
        })

      this.pendingRecordByKey[key] = request
      return request
    },

    async createRecord(entity: string, data: RecordData): Promise<RecordData> {
      const state = this.ensureRecord(entity, 'new')
      state.saving = true
      state.saveError = null

      try {
        const record = await createRecordRequest(entity, data)
        state.record = record
        state.status = 'ready'
        state.error = null
        this.cacheNamedRecord(entity, record)
        this.markEntityStale(entity)
        return record
      } catch (error: unknown) {
        const normalized = storeError(error, 'Studio could not create this record.')
        state.saveError = normalized
        throw error
      } finally {
        state.saving = false
      }
    },

    async updateRecord(entity: string, recordName: string, id: string | number, data: RecordData): Promise<RecordData> {
      const state = this.ensureRecord(entity, recordName)
      state.saving = true
      state.saveError = null

      try {
        const record = await updateRecordRequest(entity, id, data)
        state.record = record
        state.status = 'ready'
        state.error = null
        this.cacheNamedRecord(entity, record)
        this.markEntityStale(entity)
        return record
      } catch (error: unknown) {
        const normalized = storeError(error, 'Studio could not save this record.')
        state.saveError = normalized
        throw error
      } finally {
        state.saving = false
      }
    },

    async updateSingleRecord(entity: string, data: RecordData): Promise<RecordData> {
      const state = this.ensureRecord(entity, singleRecordKey)
      state.saving = true
      state.saveError = null

      try {
        const record = await updateSingleRecordRequest(entity, data)
        state.record = record
        state.status = 'ready'
        state.error = null
        this.cacheNamedRecord(entity, record)
        this.markEntityStale(entity)
        return record
      } catch (error: unknown) {
        const normalized = storeError(error, 'Studio could not save these settings.')
        state.saveError = normalized
        throw error
      } finally {
        state.saving = false
      }
    },

    async deleteRecord(entity: string, recordName: string, id: string | number): Promise<void> {
      const state = this.ensureRecord(entity, recordName)
      state.saving = true
      state.saveError = null

      try {
        await deleteRecordRequest(entity, id)
        state.record = null
        state.status = 'empty'
        state.error = null
        this.markEntityStale(entity)
      } catch (error: unknown) {
        const normalized = storeError(error, 'Studio could not delete this record.')
        state.saveError = normalized
        throw error
      } finally {
        state.saving = false
      }
    },

    cacheNamedRecord(entity: string, record: RecordData) {
      if (typeof record.name !== 'string' || record.name.length === 0) {
        return
      }

      const key = recordStateKey(entity, record.name)
      this.byRecord[key] = {
        record,
        status: 'ready',
        saving: false,
        error: null,
        saveError: null,
      }
    },

    async loadMore(entity: string): Promise<RecordEntityState> {
      const state = this.ensureEntity(entity)

      if (state.status === 'loading' || state.loadingMore || state.rows.length >= state.total || state.error) {
        return state
      }

      const activeRequest = beginRecordListRequest(entity)
      const params = recordListParams(state, state.rows.length)

      state.loadingMore = true
      state.loadMoreError = null

      try {
        const result = await listRecords(entity, params, { signal: activeRequest.controller.signal })
        if (!isCurrentRecordListRequest(entity, activeRequest)) {
          return state
        }

        state.rows = [...state.rows, ...result.data]
        state.total = result.meta.total ?? state.rows.length
        state.status = state.rows.length === 0 ? 'empty' : 'ready'
        state.stale = false
      } catch (error: unknown) {
        if (!isCurrentRecordListRequest(entity, activeRequest) || isAbortError(error)) {
          return state
        }

        const normalized = storeError(error, 'Studio could not load more records.')
        state.loadMoreError = normalized
      } finally {
        if (isCurrentRecordListRequest(entity, activeRequest)) {
          state.loadingMore = false
          finishRecordListRequest(entity, activeRequest)
        }
      }

      return state
    },

    async setPageSize(entity: string, pageSize: number): Promise<RecordEntityState> {
      await this.ensureRecordListPolicy()
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

    async setListQuery(entity: string, query: RecordListRouteState): Promise<RecordEntityState> {
      const state = this.ensureEntity(entity)
      const nextFilters = query.filters.map((filter) => ({ ...filter }))
      const same = sortsEqual(state.sort, query.sort) && filtersEqual(state.filters, nextFilters)
      if (!same) {
        state.sort = query.sort
        state.filters = nextFilters
        state.selectedRowKeys = []
        state.loadMoreError = null
        state.stale = true
      }

      return this.loadInitial(entity, { force: !same && state.status !== 'idle' })
    },

    async setFilters(entity: string, filters: RecordListFilter[]): Promise<RecordEntityState> {
      const state = this.ensureEntity(entity)
      const nextFilters = filters.map((filter) => ({ ...filter }))
      if (filtersEqual(state.filters, nextFilters)) {
        return state
      }

      state.filters = nextFilters
      state.selectedRowKeys = []
      state.loadMoreError = null
      state.stale = true
      return this.loadInitial(entity, { force: true })
    },

    resetFilters(entity: string) {
      const state = this.ensureEntity(entity)
      state.filters = []
      state.selectedRowKeys = []
      state.loadMoreError = null
      state.stale = true
    },

    setGlobalPageSize(pageSize: number) {
      const policy = usePlatformStore().recordListPolicy
      if (!policy) {
        return
      }
      if (!isAllowedRecordPageSize(pageSize, policy['page-sizes'])) {
        return
      }

      this.pageSize = pageSize
      writeStoredPageSize(pageSize, policy)

      Object.entries(this.byEntity).forEach(([entity, state]) => {
        if (state.pageSize === pageSize) {
          return
        }

        cancelRecordListRequest(entity)
        this.pendingInitialByEntity[entity] = undefined
        state.pageSize = pageSize
        state.loadingMore = false
        state.selectedRowKeys = []
        state.stale = true
      })
    },

    setSelectedRowKeys(entity: string, keys: DataTableRowKey[]) {
      const state = this.ensureEntity(entity)
      state.selectedRowKeys = keys
    },

    markEntityStale(entity: string) {
      const state = this.byEntity[entity]
      if (!state) {
        return
      }

      state.stale = true
    },

    resetRecordForm(entity: string, recordName: string) {
      const key = recordStateKey(entity, recordName)
      this.byRecord[key] = newRecordFormState()
      this.pendingRecordByKey[key] = undefined
    },

    resetEntity(entity: string) {
      cancelRecordListRequest(entity)
      this.byEntity[entity] = newRecordEntityState(this.pageSize)
      this.pendingInitialByEntity[entity] = undefined
    },
  },
})
