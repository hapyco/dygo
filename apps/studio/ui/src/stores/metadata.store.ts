import { defineStore } from 'pinia'

import {
  getEntityMeta,
  listEntities,
  MetadataApiError,
  type MetadataEntity,
  type MetadataEntityMeta,
} from '@/features/metadata/metadata.api'
import { findEntityByRouteSlug, metadataCacheSlugs } from './metadata.identity'
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
  entityMetaBySlug: Record<string, MetadataEntityMeta>
  metaStatusBySlug: Record<string, LoadStatus>
  metaErrorBySlug: Record<string, StoreError | null>
  pendingMetaBySlug: Record<string, Promise<MetadataEntityMeta | null> | undefined>
}

export const useMetadataStore = defineStore('metadata', {
  state: (): MetadataState => ({
    entities: [],
    entitiesStatus: 'idle',
    entitiesError: null,
    entitiesLoaded: false,
    pendingEntities: null,
    entityMetaBySlug: {},
    metaStatusBySlug: {},
    metaErrorBySlug: {},
    pendingMetaBySlug: {},
  }),

  getters: {
    entityByRouteSlug: (state) => (slug: string): MetadataEntity | undefined => (
      findEntityByRouteSlug(state.entities, slug)
    ),

    entityMeta: (state) => (entity: string): MetadataEntityMeta | null => (
      state.entityMetaBySlug[entity] ?? null
    ),

    entityMetaStatus: (state) => (entity: string): LoadStatus => (
      state.metaStatusBySlug[entity] ?? 'idle'
    ),

    entityMetaError: (state) => (entity: string): StoreError | null => (
      state.metaErrorBySlug[entity] ?? null
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
      if (this.entityMetaBySlug[entity] && !options.force) {
        return this.entityMetaBySlug[entity]
      }

      const pending = this.pendingMetaBySlug[entity]
      if (pending && !options.force) {
        return pending
      }

      this.metaStatusBySlug[entity] = 'loading'
      this.metaErrorBySlug[entity] = null

      const request = getEntityMeta(entity)
        .then((meta) => {
          this.setEntityMeta(meta, entity)
          return meta
        })
        .catch((error: unknown) => {
          const normalized = storeError(error, 'Studio could not load entity metadata.')
          this.metaErrorBySlug[entity] = normalized
          this.metaStatusBySlug[entity] = error instanceof MetadataApiError ? statusForError(normalized) : 'error'
          return null
        })
        .finally(() => {
          this.pendingMetaBySlug[entity] = undefined
        })

      this.pendingMetaBySlug[entity] = request
      return request
    },

    setEntityMeta(meta: MetadataEntityMeta, requestedSlug?: string) {
      metadataCacheSlugs(meta, requestedSlug).forEach((slug) => {
        this.entityMetaBySlug[slug] = meta
        this.metaStatusBySlug[slug] = 'ready'
        this.metaErrorBySlug[slug] = null
      })
    },
  },
})
