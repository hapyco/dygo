<script setup lang="ts">
import { computed, watch, type Component } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import * as LucideIcons from '@lucide/vue'

import { routeParam, RouteName } from '@/router/routes'
import Shell from '@/shell/Shell.vue'
import type { ShellNavItem } from '@/shell/types'
import { useAuthStore } from '@/stores/auth.store'
import { useMetadataStore } from '@/stores/metadata.store'

const route = useRoute()
const authStore = useAuthStore()
const metadataStore = useMetadataStore()

const usesShell = computed(() => !route.meta.public)
const currentEntity = computed(() => {
  const value = route.params.entity
  if (typeof value !== 'string' && !Array.isArray(value)) {
    return ''
  }

  return routeParam(value)
})

const lucideIconRegistry = LucideIcons as unknown as Record<string, Component | undefined>
const fallbackEntityIcon = LucideIcons.Box as Component

const navItems = computed<ShellNavItem[]>(() => {
  return metadataStore.entities
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

    await metadataStore.loadEntities()
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

function iconForEntity(icon?: string): Component {
  const key = icon?.trim()
  if (!key) {
    return fallbackEntityIcon
  }

  return lucideIconRegistry[key] ?? lucideIconRegistry[toPascalIconName(key)] ?? fallbackEntityIcon
}

function toPascalIconName(value: string): string {
  return value
    .split(/[-_\s]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join('')
}

function humanizeEntity(value: string): string {
  return value
    .replace(/[-_]+/g, ' ')
    .replace(/\b\w/g, (letter) => letter.toUpperCase())
}
</script>

<template>
  <RouterView v-if="!usesShell" />
  <Shell v-else :user-name="userName" :nav-items="navItems">
    <template #sidebar>
      <div v-if="metadataStore.entitiesStatus === 'loading'" class="studio-entity-nav-state">
        Loading entities
      </div>
      <div v-else-if="metadataStore.entitiesError" class="studio-entity-nav-state">
        {{ metadataStore.entitiesError.message }}
      </div>
      <div v-else-if="metadataStore.entities.length === 0" class="studio-entity-nav-state">
        No entities yet
      </div>
    </template>

    <RouterView />
  </Shell>
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
