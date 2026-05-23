import { defineStore } from 'pinia'

import {
  getEntityMeta,
  listEntities,
  MetadataApiError,
  type MetadataEntity,
  type MetadataEntityMeta,
} from '@/features/metadata/metadata.api'
import { statusForError, storeError, type LoadStatus, type StoreError } from './status'

type LoadOptions = {
  force?: boolean
}

type MetadataState = {
  entities: MetadataEntity[]
  entitiesStatus: LoadStatus
  entitiesError: StoreError | null
  entitiesLoaded: boolean
  pendingEntities: Promise<MetadataEntity[]> | null
  entityMetaByKey: Record<string, MetadataEntityMeta>
  metaStatusByKey: Record<string, LoadStatus>
  metaErrorByKey: Record<string, StoreError | null>
  pendingMetaByKey: Record<string, Promise<MetadataEntityMeta | null> | undefined>
}

export const useMetadataStore = defineStore('metadata', {
  state: (): MetadataState => ({
    entities: [],
    entitiesStatus: 'idle',
    entitiesError: null,
    entitiesLoaded: false,
    pendingEntities: null,
    entityMetaByKey: {},
    metaStatusByKey: {},
    metaErrorByKey: {},
    pendingMetaByKey: {},
  }),

  getters: {
    entityByRouteSlug: (state) => (slug: string): MetadataEntity | undefined => (
      state.entities.find((entity) => entity.slug === slug || entity.key === slug)
    ),

    entityMeta: (state) => (entity: string): MetadataEntityMeta | null => (
      state.entityMetaByKey[entity] ?? null
    ),

    entityMetaStatus: (state) => (entity: string): LoadStatus => (
      state.metaStatusByKey[entity] ?? 'idle'
    ),

    entityMetaError: (state) => (entity: string): StoreError | null => (
      state.metaErrorByKey[entity] ?? null
    ),
  },

  actions: {
    async loadEntities(options: LoadOptions = {}): Promise<MetadataEntity[]> {
      if (this.entitiesLoaded && !options.force) {
        return this.entities
      }

      if (this.pendingEntities && !options.force) {
        return this.pendingEntities
      }

      this.entitiesStatus = 'loading'
      this.entitiesError = null

      this.pendingEntities = listEntities()
        .then((entities) => {
          this.entities = entities
          this.entitiesLoaded = true
          this.entitiesStatus = entities.length === 0 ? 'empty' : 'ready'
          this.entitiesError = null
          return entities
        })
        .catch((error: unknown) => {
          const normalized = storeError(error, 'Studio could not load entities.')
          this.entities = []
          this.entitiesLoaded = true
          this.entitiesError = normalized
          this.entitiesStatus = error instanceof MetadataApiError ? statusForError(normalized) : 'error'
          return []
        })
        .finally(() => {
          this.pendingEntities = null
        })

      return this.pendingEntities
    },

    async loadEntityMeta(entity: string, options: LoadOptions = {}): Promise<MetadataEntityMeta | null> {
      if (this.entityMetaByKey[entity] && !options.force) {
        return this.entityMetaByKey[entity]
      }

      const pending = this.pendingMetaByKey[entity]
      if (pending && !options.force) {
        return pending
      }

      this.metaStatusByKey[entity] = 'loading'
      this.metaErrorByKey[entity] = null

      const request = getEntityMeta(entity)
        .then((meta) => {
          this.setEntityMeta(meta, entity)
          return meta
        })
        .catch((error: unknown) => {
          const normalized = storeError(error, 'Studio could not load entity metadata.')
          this.metaErrorByKey[entity] = normalized
          this.metaStatusByKey[entity] = error instanceof MetadataApiError ? statusForError(normalized) : 'error'
          return null
        })
        .finally(() => {
          this.pendingMetaByKey[entity] = undefined
        })

      this.pendingMetaByKey[entity] = request
      return request
    },

    setEntityMeta(meta: MetadataEntityMeta, requestedKey?: string) {
      const keys = new Set([requestedKey, meta.name, meta.key, meta.slug].filter(Boolean) as string[])

      keys.forEach((key) => {
        this.entityMetaByKey[key] = meta
        this.metaStatusByKey[key] = 'ready'
        this.metaErrorByKey[key] = null
      })
    },
  },
})
