import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import { useQuery } from '@tanstack/vue-query'

import { getEntityMeta, listEntities } from './metadata.api'

type QueryToggle = MaybeRefOrGetter<boolean>

export const metadataEntitiesQueryKey = ['metadata', 'entities'] as const

export function metadataEntityMetaQueryKey(entity: string) {
  return ['metadata', 'entity-meta', entity] as const
}

export function metadataEntitiesQueryOptions() {
  return {
    queryKey: metadataEntitiesQueryKey,
    queryFn: ({ signal }: { signal?: AbortSignal }) => listEntities({ signal }),
  }
}

export function metadataEntityMetaQueryOptions(entity: string) {
  return {
    queryKey: metadataEntityMetaQueryKey(entity),
    queryFn: ({ signal }: { signal?: AbortSignal }) => getEntityMeta(entity, { signal }),
  }
}

export function useMetadataEntitiesQuery(options: { enabled?: QueryToggle } = {}) {
  return useQuery({
    ...metadataEntitiesQueryOptions(),
    enabled: queryEnabled(options.enabled),
  })
}

export function useMetadataEntityMetaQuery(entity: MaybeRefOrGetter<string>, options: { enabled?: QueryToggle } = {}) {
  const currentEntity = computed(() => toValue(entity).trim())

  return useQuery({
    queryKey: computed(() => metadataEntityMetaQueryKey(currentEntity.value)),
    queryFn: ({ signal }) => getEntityMeta(currentEntity.value, { signal }),
    enabled: computed(() => currentEntity.value !== '' && toValue(options.enabled ?? true)),
  })
}

function queryEnabled(enabled: QueryToggle | undefined) {
  return computed(() => toValue(enabled ?? true))
}
