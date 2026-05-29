import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import { useQuery } from '@tanstack/vue-query'

import { getPlatformConfig } from './platform.api'

type QueryToggle = MaybeRefOrGetter<boolean>

export const platformConfigQueryKey = ['platform', 'config'] as const

export function platformConfigQueryOptions() {
  return {
    queryKey: platformConfigQueryKey,
    queryFn: ({ signal }: { signal?: AbortSignal }) => getPlatformConfig({ signal }),
  }
}

export function usePlatformConfigQuery(options: { enabled?: QueryToggle } = {}) {
  return useQuery({
    ...platformConfigQueryOptions(),
    enabled: computed(() => toValue(options.enabled ?? true)),
  })
}
