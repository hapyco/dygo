<script setup lang="ts">
import { computed, nextTick, ref, watch, type Component } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter, type RouteLocationNormalizedLoaded } from 'vue-router'
import { onKeyStroke } from '@vueuse/core'
import * as LucideIcons from '@lucide/vue'
import {
  Box,
  Clock,
  Command,
  FilePlus2,
  Home,
  LogOut,
  RefreshCw,
  Search,
} from '@lucide/vue'

import { queryClient } from '@/app/query'
import { reloadStudioApp } from '@/app/reload'
import type { MetadataEntity } from '@/features/metadata/metadata.api'
import { useMetadataEntitiesQuery } from '@/features/metadata/metadata.query'
import { routeParam, RouteName } from '@/router/routes'
import { useAuthStore } from '@/stores/auth.store'
import { useBootStore } from '@/stores/boot.store'
import { findEntityByRouteSlug } from '@/stores/metadata.identity'
import { useNavigationStore, type RecentPage } from '@/stores/navigation.store'

type CommandItem = {
  id: string
  label: string
  detail: string
  keywords: string
  icon: Component
  run: () => void | Promise<void>
  disabled?: boolean
}

type CommandGroup = {
  key: string
  label: string
  items: CommandItem[]
}

const navigationStore = useNavigationStore()
const bootStore = useBootStore()
const authStore = useAuthStore()
const route = useRoute()
const router = useRouter()
const { commandMenuOpen, recentPages } = storeToRefs(navigationStore)
const searchInput = ref<HTMLInputElement | null>(null)
const query = ref('')
const activeItemId = ref('')
const runningAppAction = ref(false)
const lucideIconRegistry = LucideIcons as unknown as Record<string, Component | undefined>
const metadataEntitiesQuery = useMetadataEntitiesQuery({
  enabled: computed(() => Boolean(authStore.currentUser)),
})
const metadataEntities = computed(() => metadataEntitiesQuery.data.value ?? [])

const searchableEntities = computed(() => (
  metadataEntities.value
    .filter((entity) => !entity['is-collection'] && entity.slug)
    .slice()
    .sort((first, second) => entityLabel(first).localeCompare(entityLabel(second)))
))

const normalizedQuery = computed(() => normalizeSearch(query.value))
const hasQuery = computed(() => normalizedQuery.value.length > 0)

const pageCommandItems = computed<CommandItem[]>(() => (
  searchableEntities.value.map((entity) => {
    const slug = entity.slug as string
    const label = entityLabel(entity)

    return {
      id: `page:${slug}`,
      label,
      detail: entity['is-single'] ? 'Open single record' : 'Open record list',
      keywords: `${label} ${slug} ${entity.key} ${entity.name} ${entity.app.label}`,
      icon: iconForEntity(entity.icon),
      run: () => navigateTo(`/${slug}`),
    }
  })
))

const createCommandItems = computed<CommandItem[]>(() => (
  searchableEntities.value
    .filter((entity) => !entity['is-single'] && !entity['is-system'])
    .map((entity) => {
      const slug = entity.slug as string
      const label = entityLabel(entity)

      return {
        id: `create:${slug}`,
        label: `New ${label}`,
        detail: 'Create record',
        keywords: `new create add ${label} ${slug} ${entity.key} ${entity.name}`,
        icon: FilePlus2,
        run: () => navigateTo({ name: RouteName.RecordNew, params: { entity: slug } }),
      }
    })
))

const appActionItems = computed<CommandItem[]>(() => [
  {
    id: 'app:home',
    label: 'Go home',
    detail: 'Open default home',
    keywords: 'go home start default',
    icon: Home,
    run: goHome,
  },
  {
    id: 'app:reload',
    label: 'Reload app',
    detail: 'Refresh boot and cached Studio data',
    keywords: 'reload refresh boot cache app',
    icon: RefreshCw,
    disabled: runningAppAction.value,
    run: reloadApp,
  },
  {
    id: 'app:logout',
    label: 'Log out',
    detail: 'End current session',
    keywords: 'logout log out sign out session',
    icon: LogOut,
    disabled: runningAppAction.value,
    run: logout,
  },
])

const recentCommandItems = computed<CommandItem[]>(() => (
  recentPages.value.map((page) => ({
    id: `recent:${page.path}`,
    label: page.label,
    detail: page.detail,
    keywords: `${page.label} ${page.detail} ${page.path}`,
    icon: Clock,
    run: () => navigateTo(page.path),
  }))
))

const commandGroups = computed<CommandGroup[]>(() => {
  if (!hasQuery.value) {
    return recentCommandItems.value.length > 0
      ? [{ key: 'recent', label: 'Recent', items: recentCommandItems.value }]
      : []
  }

  return [
    { key: 'pages', label: 'Pages', items: filterItems(pageCommandItems.value, normalizedQuery.value) },
    { key: 'create', label: 'Create', items: filterItems(createCommandItems.value, normalizedQuery.value) },
    { key: 'app-actions', label: 'App actions', items: filterItems(appActionItems.value, normalizedQuery.value) },
  ].filter((group) => group.items.length > 0)
})

const visibleItems = computed(() => commandGroups.value.flatMap((group) => group.items))
const emptyMessage = computed(() => (hasQuery.value ? 'No matching commands' : 'No recent pages yet'))
const activeDescendantId = computed(() => {
  const activeItem = visibleItems.value.find((item) => item.id === activeItemId.value)
  return activeItem ? itemDomId(activeItem) : undefined
})

const currentRecentPage = computed(() => recentPageForRoute(route))

watch(
  currentRecentPage,
  (page) => {
    navigationStore.rememberRecentPage(page)
  },
  { immediate: true },
)

watch(
  visibleItems,
  (items) => {
    if (items.some((item) => item.id === activeItemId.value)) {
      return
    }

    activeItemId.value = items[0]?.id ?? ''
  },
  { immediate: true },
)

watch(
  commandMenuOpen,
  async (open) => {
    if (!open) {
      query.value = ''
      return
    }

    await nextTick()
    searchInput.value?.focus()
  },
)

onKeyStroke((event) => {
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault()
    if (commandMenuOpen.value) {
      closeMenu()
      return
    }

    void openMenu()
    return
  }

  if (event.key === 'Escape' && commandMenuOpen.value) {
    event.preventDefault()
    closeMenu()
  }
}, { passive: false })

async function openMenu() {
  navigationStore.openCommandMenu()
  await nextTick()
  searchInput.value?.focus()
}

function closeMenu() {
  navigationStore.closeCommandMenu()
}

function handleInputKeydown(event: KeyboardEvent) {
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    moveActiveItem(1)
    return
  }

  if (event.key === 'ArrowUp') {
    event.preventDefault()
    moveActiveItem(-1)
    return
  }

  if (event.key === 'Enter') {
    event.preventDefault()
    const activeItem = visibleItems.value.find((item) => item.id === activeItemId.value)
    if (activeItem) {
      void runCommand(activeItem)
    }
    return
  }

  if (event.key === 'Escape') {
    event.preventDefault()
    closeMenu()
  }
}

function moveActiveItem(direction: 1 | -1) {
  const items = visibleItems.value
  if (items.length === 0) {
    activeItemId.value = ''
    return
  }

  const currentIndex = Math.max(0, items.findIndex((item) => item.id === activeItemId.value))
  const nextIndex = (currentIndex + direction + items.length) % items.length
  activeItemId.value = items[nextIndex].id
}

async function runCommand(item: CommandItem) {
  if (item.disabled) {
    return
  }

  closeMenu()
  await item.run()
}

async function navigateTo(to: string | { name: string, params?: Record<string, string> }) {
  await router.push(to)
}

async function goHome() {
  await router.push(normalizeHome(bootStore.defaults?.home))
}

async function reloadApp() {
  if (runningAppAction.value) {
    return
  }

  runningAppAction.value = true
  try {
    await reloadStudioApp(router)
  } finally {
    runningAppAction.value = false
  }
}

async function logout() {
  if (runningAppAction.value) {
    return
  }

  runningAppAction.value = true
  try {
    await authStore.logout()
    queryClient.clear()
    await router.replace({ name: RouteName.Login })
  } finally {
    runningAppAction.value = false
  }
}

function filterItems(items: CommandItem[], search: string): CommandItem[] {
  const tokens = search.split(' ').filter(Boolean)
  return items.filter((item) => {
    const haystack = normalizeSearch(`${item.label} ${item.detail} ${item.keywords}`)
    return tokens.every((token) => haystack.includes(token))
  })
}

function recentPageForRoute(currentRoute: RouteLocationNormalizedLoaded): RecentPage | null {
  if (currentRoute.name === RouteName.Home) {
    return {
      path: '/',
      label: 'Home',
      detail: 'Default start page',
    }
  }

  if (currentRoute.name === RouteName.EntityRecords) {
    const entity = routeParam(currentRoute.params.entity as string | string[])
    const meta = findEntityByRouteSlug(metadataEntities.value, entity)

    return {
      path: currentRoute.path,
      label: meta ? entityLabel(meta) : humanizeEntity(entity),
      detail: meta?.['is-single'] ? 'Single record' : 'Record list',
    }
  }

  if (currentRoute.name === RouteName.RecordDetail) {
    const entity = routeParam(currentRoute.params.entity as string | string[])
    const recordName = routeParam(currentRoute.params.recordName as string | string[])
    const meta = findEntityByRouteSlug(metadataEntities.value, entity)
    const label = meta ? entityLabel(meta) : humanizeEntity(entity)

    return {
      path: currentRoute.path,
      label: `${label} / ${recordName}`,
      detail: 'Record',
    }
  }

  return null
}

function normalizeHome(value: unknown): string {
  if (typeof value !== 'string') {
    return '/'
  }

  const home = value.trim()
  if (home === '' || !home.startsWith('/') || home.startsWith('//')) {
    return '/'
  }

  return home
}

function entityLabel(entity: MetadataEntity): string {
  return entity.label || humanizeEntity(entity.slug ?? entity.key)
}

function iconForEntity(icon?: string): Component {
  const key = icon?.trim()
  if (!key) {
    return Box
  }

  return lucideIconRegistry[key] ?? lucideIconRegistry[toPascalIconName(key)] ?? Box
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

function normalizeSearch(value: string): string {
  return value.trim().replace(/\s+/g, ' ').toLowerCase()
}

function itemDomId(item: CommandItem): string {
  return `studio-command-menu-item-${item.id.replace(/[^a-zA-Z0-9_-]/g, '-')}`
}
</script>

<template>
  <div class="studio-command-menu">
    <button
      class="studio-command-menu__trigger"
      type="button"
      aria-haspopup="dialog"
      :aria-expanded="commandMenuOpen"
      @click="openMenu"
    >
      <Search class="studio-command-menu__search-icon" :size="14" :stroke-width="1.8" aria-hidden="true" />
      <span class="studio-command-menu__placeholder">Search</span>
      <span class="studio-command-menu__shortcut" aria-hidden="true">
        <Command :size="12" :stroke-width="1.9" />
        <kbd>K</kbd>
      </span>
      <span class="studio-command-menu__shortcut-label">Command K</span>
    </button>

    <Teleport to="body">
      <div
        v-if="commandMenuOpen"
        class="studio-command-menu__overlay"
        role="presentation"
        @click.self="closeMenu"
      >
        <section
          class="studio-command-menu__dialog"
          role="dialog"
          aria-modal="true"
          aria-label="Command menu"
        >
          <div class="studio-command-menu__input-wrap">
            <Search class="studio-command-menu__dialog-icon" :size="16" :stroke-width="1.8" aria-hidden="true" />
            <input
              ref="searchInput"
              v-model="query"
              class="studio-command-menu__input"
              type="search"
              placeholder="Search or type a command"
              role="combobox"
              aria-controls="studio-command-menu-list"
              :aria-expanded="commandMenuOpen"
              :aria-activedescendant="activeDescendantId"
              @keydown="handleInputKeydown"
            >
            <span class="studio-command-menu__dialog-shortcut" aria-hidden="true">
              <Command :size="12" :stroke-width="1.9" />
              <kbd>K</kbd>
            </span>
          </div>

          <div
            id="studio-command-menu-list"
            class="studio-command-menu__list"
            role="listbox"
            aria-label="Commands"
          >
            <template v-if="commandGroups.length > 0">
              <section
                v-for="group in commandGroups"
                :key="group.key"
                class="studio-command-menu__group"
              >
                <div class="studio-command-menu__group-label">{{ group.label }}</div>

                <button
                  v-for="item in group.items"
                  :id="itemDomId(item)"
                  :key="item.id"
                  class="studio-command-menu__item"
                  :class="{ 'studio-command-menu__item--active': item.id === activeItemId }"
                  type="button"
                  role="option"
                  :aria-selected="item.id === activeItemId"
                  :disabled="item.disabled"
                  @mouseenter="activeItemId = item.id"
                  @click="runCommand(item)"
                >
                  <component
                    :is="item.icon"
                    class="studio-command-menu__item-icon"
                    :size="16"
                    :stroke-width="1.8"
                    aria-hidden="true"
                  />
                  <span class="studio-command-menu__item-copy">
                    <span class="studio-command-menu__item-label">{{ item.label }}</span>
                    <span class="studio-command-menu__item-detail">{{ item.detail }}</span>
                  </span>
                </button>
              </section>
            </template>

            <div v-else class="studio-command-menu__empty">
              {{ emptyMessage }}
            </div>
          </div>

          <div class="studio-command-menu__footer" aria-hidden="true">
            <span class="studio-command-menu__hint">
              <kbd>Up</kbd>
              <kbd>Down</kbd>
              <span>navigate</span>
            </span>
            <span class="studio-command-menu__hint">
              <kbd>Enter</kbd>
              <span>select</span>
            </span>
            <span class="studio-command-menu__hint">
              <span class="studio-command-menu__hint-shortcut">
                <Command :size="12" :stroke-width="1.9" />
                <kbd>K</kbd>
              </span>
              <span>close</span>
            </span>
            <span class="studio-command-menu__hint">
              <kbd>Esc</kbd>
              <span>close</span>
            </span>
          </div>
        </section>
      </div>
    </Teleport>
  </div>
</template>

<style>
.studio-command-menu {
  min-width: 0;
}

.studio-command-menu__trigger {
  display: flex;
  width: 100%;
  height: var(--studio-control-height-xs);
  min-width: 0;
  align-items: center;
  gap: 8px;
  border: 1px solid var(--studio-border);
  border-radius: var(--studio-radius-control);
  background: var(--studio-control-bg);
  color: var(--studio-text-subtle);
  padding: 0 6px 0 10px;
  text-align: left;
  box-shadow: var(--studio-shadow-control);
  transition:
    background-color 160ms ease,
    border-color 160ms ease,
    box-shadow 160ms ease;
}

.studio-command-menu__trigger:hover {
  border-color: var(--studio-border-strong);
  background: var(--studio-control-bg-hover);
}

.studio-command-menu__trigger:focus-visible {
  border-color: var(--studio-focus);
  outline: none;
  box-shadow: var(--studio-focus-ring);
}

.studio-command-menu__search-icon {
  flex: 0 0 auto;
}

.studio-command-menu__placeholder {
  min-width: 0;
  flex: 1 1 auto;
  overflow: hidden;
  font-size: 12px;
  font-weight: 500;
  line-height: 1;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.studio-command-menu__shortcut,
.studio-command-menu__dialog-shortcut,
.studio-command-menu__hint-shortcut {
  display: inline-flex;
  height: 20px;
  flex: 0 0 auto;
  align-items: center;
  gap: 3px;
  border: 1px solid var(--studio-border);
  border-radius: 5px;
  background: var(--studio-neutral-soft);
  color: var(--studio-text-muted);
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
  padding: 0 5px;
}

.studio-command-menu__shortcut kbd,
.studio-command-menu__dialog-shortcut kbd,
.studio-command-menu__hint-shortcut kbd,
.studio-command-menu__hint kbd {
  font: inherit;
}

.studio-command-menu__shortcut-label {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
  white-space: nowrap;
}

.studio-command-menu__overlay {
  position: fixed;
  z-index: 60;
  inset: 0;
  display: grid;
  place-items: start center;
  background: oklch(0.225 0.018 246 / 0.18);
  padding: 86px 18px 18px;
}

.studio-command-menu__dialog {
  display: grid;
  width: min(640px, 100%);
  max-height: min(620px, calc(100vh - 120px));
  overflow: hidden;
  border: 1px solid var(--studio-border);
  border-radius: var(--studio-radius-panel);
  background: var(--studio-surface);
  box-shadow: var(--studio-shadow-sheet);
}

.studio-command-menu__input-wrap {
  display: flex;
  height: 52px;
  align-items: center;
  gap: 10px;
  border-bottom: 1px solid var(--studio-border);
  padding: 0 10px 0 16px;
}

.studio-command-menu__dialog-icon {
  flex: 0 0 auto;
  color: var(--studio-text-subtle);
}

.studio-command-menu__input {
  min-width: 0;
  flex: 1 1 auto;
  border: 0;
  background: transparent;
  color: var(--studio-text);
  font-size: 15px;
  font-weight: 500;
  outline: none;
}

.studio-command-menu__input::placeholder {
  color: var(--studio-text-subtle);
}

.studio-command-menu__list {
  display: grid;
  align-content: start;
  gap: 10px;
  min-height: 220px;
  max-height: 480px;
  overflow-y: auto;
  padding: 10px;
}

.studio-command-menu__group {
  display: grid;
  gap: 3px;
}

.studio-command-menu__group-label {
  color: var(--studio-text-subtle);
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
  padding: 7px 8px 5px;
  text-transform: uppercase;
}

.studio-command-menu__item {
  display: grid;
  min-height: 42px;
  grid-template-columns: 22px minmax(0, 1fr);
  align-items: center;
  gap: 10px;
  border: 0;
  border-radius: var(--studio-radius-control);
  background: transparent;
  color: var(--studio-text-muted);
  font: inherit;
  padding: 5px 9px;
  text-align: left;
}

.studio-command-menu__item:hover,
.studio-command-menu__item:focus-visible,
.studio-command-menu__item--active {
  background: var(--studio-surface-raised);
  color: var(--studio-text);
  outline: none;
}

.studio-command-menu__item:disabled {
  color: var(--studio-text-subtle);
  cursor: default;
}

.studio-command-menu__item-icon {
  justify-self: center;
}

.studio-command-menu__item-copy {
  display: flex;
  min-width: 0;
  align-items: baseline;
  gap: 8px;
}

.studio-command-menu__item-label {
  min-width: 0;
  overflow: hidden;
  color: inherit;
  font-size: 13px;
  font-weight: 700;
  line-height: 1.2;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.studio-command-menu__item-detail {
  flex: 0 0 auto;
  color: var(--studio-text-subtle);
  font-size: 12px;
  font-weight: 600;
  line-height: 1.2;
}

.studio-command-menu__empty {
  display: grid;
  min-height: 180px;
  place-items: center;
  color: var(--studio-text-subtle);
  font-size: 13px;
  font-weight: 600;
}

.studio-command-menu__footer {
  display: flex;
  min-height: 42px;
  flex-wrap: wrap;
  align-items: center;
  gap: 14px;
  border-top: 1px solid var(--studio-border);
  color: var(--studio-text-muted);
  font-size: 12px;
  font-weight: 600;
  padding: 7px 12px;
}

.studio-command-menu__hint {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}

.studio-command-menu__hint kbd {
  display: inline-flex;
  height: 20px;
  min-width: 25px;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--studio-border);
  border-radius: 5px;
  background: var(--studio-neutral-soft);
  color: var(--studio-text-muted);
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
  padding: 0 5px;
}

@media (max-width: 720px) {
  .studio-command-menu__dialog {
    max-height: calc(100vh - 72px);
  }

  .studio-command-menu__overlay {
    padding-top: 54px;
  }

  .studio-command-menu__item-copy {
    display: grid;
    gap: 2px;
  }

  .studio-command-menu__item-detail {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
}
</style>
