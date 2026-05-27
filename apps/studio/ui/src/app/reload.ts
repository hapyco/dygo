import type { Router } from 'vue-router'

import { RouteName } from '@/router/routes'
import { useAuthStore } from '@/stores/auth.store'
import { useBootStore } from '@/stores/boot.store'
import { useMetadataStore } from '@/stores/metadata.store'
import { useNavigationStore } from '@/stores/navigation.store'
import { pinia } from '@/stores/pinia'
import { usePlatformStore } from '@/stores/platform.store'
import { useRecordsStore } from '@/stores/records.store'

export async function reloadStudioApp(router: Router): Promise<void> {
  const currentRoute = router.currentRoute.value
  const redirectTarget = currentRoute.fullPath
  const authStore = useAuthStore(pinia)
  const bootStore = useBootStore(pinia)
  const metadataStore = useMetadataStore(pinia)
  const navigationStore = useNavigationStore(pinia)
  const platformStore = usePlatformStore(pinia)
  const recordsStore = useRecordsStore(pinia)

  authStore.$reset()
  bootStore.$reset()
  metadataStore.$reset()
  navigationStore.$reset()
  platformStore.$reset()
  recordsStore.$reset()

  const user = await authStore.loadCurrentUser({ force: true })
  if (!user) {
    await router.replace({ name: RouteName.Login, query: { redirect: redirectTarget } })
    return
  }

  const boot = await bootStore.loadBoot({ force: true })
  await Promise.all([
    platformStore.loadPlatform({ force: true }),
    metadataStore.loadEntities({ force: true }),
  ])

  // TODO: include DB-backed preference caches here as boot grows beyond defaults.
  navigationStore.requestRouteReload()

  if (currentRoute.name === RouteName.Home) {
    const home = normalizeReloadHome(boot?.defaults.home)
    if (home !== '/') {
      await router.replace(home)
    }
  }
}

function normalizeReloadHome(value: unknown): string {
  if (typeof value !== 'string') {
    return '/'
  }

  const home = value.trim()
  if (home === '' || !home.startsWith('/') || home.startsWith('//')) {
    return '/'
  }

  return home
}
