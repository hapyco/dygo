import { defineStore } from 'pinia'

import type { DataTableRowKey, DataTableSort } from '@/design/types'
import {
  createRecord as createRecordRequest,
  getRecordByName,
  getSingleRecord,
  listRecords,
  RecordApiError,
  updateRecord as updateRecordRequest,
  updateSingleRecord as updateSingleRecordRequest,
  type RecordData,
} from '@/features/records/records.api'
import { isAllowedRecordPageSize } from '@/features/records/query'
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
        Object.values(this.byEntity).forEach((state) => {
          if (state.pageSize === nextPageSize) {
            return
          }

          state.pageSize = nextPageSize
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
      this.byEntity[entity] = newRecordEntityState(this.pageSize)
      this.pendingInitialByEntity[entity] = undefined
    },
  },
})
