<script setup lang="ts">
import { computed } from 'vue'
import { ChevronRight, Home } from '@lucide/vue'
import { useRoute } from 'vue-router'

import { RouteName } from '@/router/routes'

const route = useRoute()

const entitySlug = computed(() => {
  const entity = route.params.entity
  return typeof entity === 'string' && entity.length > 0 ? entity : ''
})

const entityCrumb = computed(() => {
  if (
    entitySlug.value
    && (route.name === RouteName.RecordNew || route.name === RouteName.RecordDetail)
  ) {
    return humanize(entitySlug.value)
  }

  return ''
})

const currentCrumb = computed(() => {
  if (route.name === RouteName.Home) {
    return ''
  }

  const entity = entitySlug.value
  if (entity.length > 0) {
    if (route.name === RouteName.RecordNew) {
      return 'New record'
    }

    if (route.name === RouteName.RecordDetail && typeof route.params.recordName === 'string') {
      return route.params.recordName
    }

    return humanize(entity)
  }

  if (typeof route.name === 'string') {
    return humanize(route.name)
  }

  return ''
})

function humanize(value: string): string {
  return value
    .replace(/[-_]+/g, ' ')
    .replace(/\b\w/g, (letter) => letter.toUpperCase())
}
</script>

<template>
  <nav class="studio-breadcrumbs" aria-label="Breadcrumb">
    <RouterLink class="studio-breadcrumbs__home" :to="{ name: RouteName.Home }" aria-label="Home">
      <Home class="studio-breadcrumbs__home-icon" :size="16" :stroke-width="1.8" aria-hidden="true" />
    </RouterLink>
    <template v-if="entityCrumb">
      <ChevronRight class="studio-breadcrumbs__separator" :size="14" :stroke-width="1.8" aria-hidden="true" />
      <RouterLink class="studio-breadcrumbs__link" :to="{ name: RouteName.EntityRecords, params: { entity: entitySlug } }">
        {{ entityCrumb }}
      </RouterLink>
    </template>
    <template v-if="currentCrumb">
      <ChevronRight class="studio-breadcrumbs__separator" :size="14" :stroke-width="1.8" aria-hidden="true" />
      <span class="studio-breadcrumbs__current" aria-current="page">{{ currentCrumb }}</span>
    </template>
  </nav>
</template>

<style scoped>
.studio-breadcrumbs {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: 6px;
}

.studio-breadcrumbs__home {
  display: inline-flex;
  width: 32px;
  height: 32px;
  align-items: center;
  justify-content: center;
  border: 1px solid transparent;
  border-radius: var(--studio-radius-control);
  background: transparent;
  color: var(--studio-text-muted);
  text-decoration: none;
  transition:
    background-color 160ms ease,
    border-color 160ms ease,
    color 160ms ease;
}

.studio-breadcrumbs__home:hover {
  background: var(--studio-surface-raised);
  color: var(--studio-text);
}

.studio-breadcrumbs__home:focus-visible {
  outline: 2px solid var(--studio-focus);
  outline-offset: 2px;
}

.studio-breadcrumbs__link {
  min-width: 0;
  overflow: hidden;
  color: var(--studio-text-muted);
  font-size: 13px;
  font-weight: 600;
  line-height: 1.15;
  text-decoration: none;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.studio-breadcrumbs__link:hover {
  color: var(--studio-text);
}

.studio-breadcrumbs__link:focus-visible {
  border-radius: 4px;
  outline: 2px solid var(--studio-focus);
  outline-offset: 2px;
}

.studio-breadcrumbs__home-icon {
  flex: 0 0 auto;
}

.studio-breadcrumbs__separator {
  flex: 0 0 auto;
  color: var(--studio-text-subtle);
}

.studio-breadcrumbs__current {
  min-width: 0;
  overflow: hidden;
  color: var(--studio-text-muted);
  font-size: 13px;
  font-weight: 600;
  line-height: 1.15;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
