<script setup lang="ts">
import { computed, onUnmounted, watch } from 'vue'
import { RouterView, useRoute } from 'vue-router'

import DialogHost from '@/features/dialogs/DialogHost.vue'
import { useDialog } from '@/features/dialogs/use-dialog'
import ToastHost from '@/features/toasts/ToastHost.vue'
import { useToast } from '@/features/toasts/use-toast'
import { setAPIDialogHandler, setAPIToastHandler } from '@/features/api/client'
import { iconForEntity } from '@/features/metadata/entity-icons'
import { routeParam, RouteName } from '@/router/routes'
import { useMetadataEntitiesQuery } from '@/features/metadata/metadata.query'
import Shell from '@/shell/Shell.vue'
import type { ShellNavItem } from '@/shell/types'
import { useAuthStore } from '@/stores/auth.store'
import { useNavigationStore } from '@/stores/navigation.store'
import { storeError } from '@/stores/status'

const route = useRoute()
const authStore = useAuthStore()
const navigationStore = useNavigationStore()
const dialog = useDialog()
const toast = useToast()

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
const shellRouteViewKey = computed(() => `${route.path}:${navigationStore.routeReloadVersion}`)
const currentEntity = computed(() => {
  const value = route.params.entity
  if (typeof value !== 'string' && !Array.isArray(value)) {
    return ''
  }

  return routeParam(value)
})
const metadataEntitiesQuery = useMetadataEntitiesQuery({
  enabled: computed(() => usesShell.value && Boolean(authStore.currentUser)),
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

watch(
  usesShell,
  async (enabled) => {
    if (!enabled) {
      return
    }

    const user = await authStore.loadCurrentUser()
    if (!user) {
      return
    }
  },
  { immediate: true },
)

function isEntityRoute(entity: string): boolean {
  if (
    route.name !== RouteName.EntityRecords
    && route.name !== RouteName.RecordNew
    && route.name !== RouteName.RecordDetail
  ) {
    return false
  }

  return currentEntity.value === entity
}

function humanizeEntity(value: string): string {
  return value
    .replace(/[-_]+/g, ' ')
    .replace(/\b\w/g, (letter) => letter.toUpperCase())
}
</script>

<template>
  <RouterView v-if="!usesShell" :key="publicRouteViewKey" />
  <Shell v-else :user-name="userName" :nav-items="navItems">
    <template #sidebar>
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

    <RouterView :key="shellRouteViewKey" />
  </Shell>
  <DialogHost />
  <ToastHost />
</template>

<style scoped>
.studio-entity-nav-state {
  margin-top: 8px;
  color: var(--studio-text-subtle);
  font-size: 12px;
  font-weight: 600;
  line-height: 1.45;
  padding: 0 10px;
}
</style>
