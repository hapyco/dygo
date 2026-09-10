<script setup lang="ts">
import { computed, defineAsyncComponent, onErrorCaptured, onUnmounted, watch } from 'vue'
import { RouterView, useRoute, useRouter } from 'vue-router'
import { LockKeyhole, RefreshCw, TriangleAlert } from '@lucide/vue'

import { Button, Spinner } from '@/design'
import { useCapturedErrors } from '@/features/debug/captured-errors'
import DialogHost from '@/features/dialogs/DialogHost.vue'
import { useDialog } from '@/features/dialogs/use-dialog'
import ToastHost from '@/features/toasts/ToastHost.vue'
import { useToast } from '@/features/toasts/use-toast'
import { setAPIDialogHandler, setAPIToastHandler } from '@/features/api/client'
import { iconForEntity } from '@/features/metadata/entity-icons'
import { studioPathIsEntity } from '@/router/current'
import { RouteName } from '@/router/routes'
import { installBootRoutes } from '@/router'
import { useMetadataEntitiesQuery } from '@/features/metadata/metadata.query'
import Shell from '@/shell/Shell.vue'
import type { ShellNavItem } from '@/shell/types'
import { useAuthStore } from '@/stores/auth.store'
import { useBootStore } from '@/stores/boot.store'
import { findEntityByRouteSlug, humanizeEntity } from '@/stores/metadata.identity'
import { useNavigationStore } from '@/stores/navigation.store'
import { storeError } from '@/stores/status'
import { tabLabelForRoute } from '@/features/tabs/tabs'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const bootStore = useBootStore()
const navigationStore = useNavigationStore()
const dialog = useDialog()
const toast = useToast()
const DebugBar = import.meta.env.DEV
  ? defineAsyncComponent(() => import('@/features/debug/DebugBar.vue'))
  : null

if (import.meta.env.DEV) {
  const { capture } = useCapturedErrors()
  onErrorCaptured(capture)
}

setAPIDialogHandler((request) => {
  void dialog.open(request)
})
setAPIToastHandler((request) => {
  toast.show(request)
})
onUnmounted(() => {
  setAPIDialogHandler(null)
  setAPIToastHandler(null)
})

const usesShell = computed(() => !route.meta.public)
const publicRouteViewKey = computed(() => `${route.fullPath}:${navigationStore.routeReloadVersion}`)
const metadataEntitiesQuery = useMetadataEntitiesQuery({
  enabled: computed(() => (
    usesShell.value
    && Boolean(authStore.currentUser)
    && bootStore.status === 'ready'
  )),
})
const metadataEntities = computed(() => metadataEntitiesQuery.data.value ?? [])
const metadataEntitiesLoading = computed(() => metadataEntitiesQuery.isPending.value)
const metadataEntitiesError = computed(() => (
  metadataEntitiesQuery.error.value
    ? storeError(metadataEntitiesQuery.error.value, 'Studio could not load entities.')
    : null
))

const navItems = computed<ShellNavItem[]>(() => {
  return metadataEntities.value
    .filter((entity) => !entity['is-collection'] && entity.slug)
    .map((entity) => {
      const slug = entity.slug as string

      return {
        label: entity.label || humanizeEntity(slug),
        to: `/${slug}`,
        icon: iconForEntity(entity.icon),
        current: isEntityRoute(slug),
      }
    })
})

const userName = computed(() => authStore.currentUser?.['full-name'] || authStore.currentUser?.email || 'Studio user')
const shellReady = computed(() => bootStore.status === 'ready')

async function retryBoot() {
  const boot = await bootStore.loadBoot({ force: true })
  if (boot) {
    installBootRoutes(router, boot.pages)
  }
}

function isEntityRoute(entity: string): boolean {
  return studioPathIsEntity(route.path, entity)
}

watch(
  () => [route.path, route.fullPath, String(route.name ?? ''), bootStore.status, authStore.currentUser?.id ?? null, metadataEntities.value] as const,
  (_current, previous) => {
    if (!usesShell.value || bootStore.status !== 'ready' || !authStore.currentUser) return
    if (route.name === RouteName.Home && typeof bootStore.defaults?.home === 'string' && bootStore.defaults.home !== '/') return
    const previousPath = previous ? previous[0] : ''
    navigationStore.syncTab({
      path: route.path,
      fullPath: route.fullPath,
      label: tabLabelForRoute({
        name: route.name,
        path: route.path,
        fullPath: route.fullPath,
        params: route.params,
      }, (slug) => {
        const entity = findEntityByRouteSlug(metadataEntities.value, slug)
        return entity?.label || humanizeEntity(slug)
      }),
    }, previousPath)
  },
  { immediate: true },
)

</script>

<template>
  <RouterView v-if="!usesShell" :key="publicRouteViewKey" />
  <div v-else class="studio-workspace">
    <Shell :user-name="userName" :nav-items="navItems" :show-sidebar="shellReady">
      <template #sidebar>
        <template v-if="shellReady">
          <div v-if="metadataEntitiesLoading" class="studio-entity-nav-state">
            Loading entities
          </div>
          <div v-else-if="metadataEntitiesError" class="studio-entity-nav-state">
            {{ metadataEntitiesError.message }}
          </div>
          <div v-else-if="metadataEntities.length === 0" class="studio-entity-nav-state">
            No entities yet
          </div>
        </template>
      </template>

      <section v-if="bootStore.status === 'loading' || bootStore.status === 'idle'" class="studio-shell-state" aria-live="polite">
        <Spinner size="sm" label="Loading Studio" />
        <p>Loading Studio</p>
      </section>

      <section v-else-if="bootStore.status === 'forbidden'" class="studio-shell-state" role="alert" aria-labelledby="studio-access-title">
        <span class="studio-shell-state__icon" aria-hidden="true">
          <LockKeyhole :size="22" :stroke-width="1.7" />
        </span>
        <p class="studio-shell-state__eyebrow">Access restricted</p>
        <h1 id="studio-access-title">Studio access is required</h1>
        <p class="studio-shell-state__message">
          Your account can sign in, but it cannot open Studio. Ask an administrator to grant Studio access.
        </p>
        <Button variant="secondary" @click="retryBoot">
          <RefreshCw aria-hidden="true" :size="14" :stroke-width="1.8" />
          Try again
        </Button>
      </section>

      <section v-else-if="bootStore.status === 'error'" class="studio-shell-state" role="alert" aria-labelledby="studio-error-title">
        <span class="studio-shell-state__icon" aria-hidden="true">
          <TriangleAlert :size="22" :stroke-width="1.7" />
        </span>
        <p class="studio-shell-state__eyebrow">Studio unavailable</p>
        <h1 id="studio-error-title">Studio could not start</h1>
        <p class="studio-shell-state__message">
          {{ bootStore.error?.message || 'Check the server, then try again.' }}
        </p>
        <Button variant="secondary" @click="retryBoot">
          <RefreshCw aria-hidden="true" :size="14" :stroke-width="1.8" />
          Try again
        </Button>
      </section>

      <RouterView v-else v-slot="{ Component }">
        <KeepAlive :max="Math.max(navigationStore.openTabs.length, 1)">
          <component :is="Component" :key="navigationStore.tabCacheKey(route.path)" />
        </KeepAlive>
      </RouterView>
    </Shell>
    <DebugBar v-if="DebugBar" />
  </div>
  <DialogHost />
  <ToastHost />
</template>

<style scoped>
.studio-workspace {
  display: flex;
  flex-direction: column;
  height: 100dvh;
  overflow: hidden;
}

.studio-workspace > :deep(.studio-shell) {
  flex: 1;
  min-height: 0;
  height: auto;
}

.studio-entity-nav-state {
  margin-top: 8px;
  color: var(--studio-text-subtle);
  font-size: 12px;
  font-weight: 600;
  line-height: 1.45;
  padding: 0 10px;
}

.studio-shell-state {
  display: grid;
  width: min(100%, 520px);
  align-content: center;
  justify-items: start;
  gap: 8px;
  min-height: 100%;
  margin: 0 auto;
  padding: 48px 28px 96px;
}

.studio-shell-state__icon {
  display: inline-flex;
  width: 42px;
  height: 42px;
  align-items: center;
  justify-content: center;
  margin-bottom: 8px;
  border: 1px solid var(--studio-border);
  border-radius: 50%;
  background: var(--studio-surface-raised);
  color: var(--studio-text-muted);
}

.studio-shell-state__eyebrow,
.studio-shell-state__message,
.studio-shell-state > p {
  margin: 0;
}

.studio-shell-state__eyebrow {
  color: var(--studio-text-subtle);
  font-size: 11px;
  font-weight: 700;
  line-height: 1.2;
}

.studio-shell-state h1 {
  margin: 0;
  color: var(--studio-text);
  font-size: 20px;
  font-weight: 700;
  line-height: 1.25;
}

.studio-shell-state__message {
  max-width: 48ch;
  margin-bottom: 10px;
  color: var(--studio-text-muted);
  font-size: 13px;
  line-height: 1.55;
}
</style>
