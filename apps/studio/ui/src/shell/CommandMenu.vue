<script setup lang="ts">
import { computed, nextTick, ref, watch, type Component } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter, type RouteLocationNormalizedLoaded } from 'vue-router'
import {
  ArrowLeft,
  Clock,
  Command,
  FilePlus2,
  LogOut,
  RefreshCw,
  Search,
} from '@lucide/vue'

import { Dialog } from '@/design'
import { queryClient } from '@/app/query'
import { pageCommands, studioCommands, globalCommands, runStudioCommand } from '@/features/commands/context'
import { bindings, commandBinding, executeCommand, nextAvailableCommandId, unavailable, shortcutLabel, ariaShortcut, type StudioCommand } from '@/features/commands/shortcuts'
import { listRecords } from '@/features/records/records.api'
import { reloadStudioApp } from '@/app/reload'
import { iconForEntity } from '@/features/metadata/entity-icons'
import type { MetadataEntity } from '@/features/metadata/metadata.api'
import { useMetadataEntitiesQuery } from '@/features/metadata/metadata.query'
import { routeParam, RouteName } from '@/router/routes'
import { useAuthStore } from '@/stores/auth.store'
import { findEntityByRouteSlug, humanizeEntity } from '@/stores/metadata.identity'
import { useNavigationStore, type RecentPage } from '@/stores/navigation.store'

type CommandItem = StudioCommand & {
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

const paletteShortcut = bindings['app:palette']?.shortcut
const navigationStore = useNavigationStore()
const authStore = useAuthStore()
const route = useRoute()
const router = useRouter()
const { commandMenuOpen, recentPages } = storeToRefs(navigationStore)
const searchInput = ref<HTMLInputElement | null>(null)
const triggerButton = ref<HTMLButtonElement | null>(null)
const query = ref('')
const activeItemId = ref('')
const runningAppAction = ref(false)
let returnFocus: HTMLElement | null = null
let afterClose: (() => void | Promise<void>) | null = null
const searchingRecords = ref(false)
const searchEntity = ref('')
const recordSearch = ref('')
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
const currentEntity = computed(() => findEntityByRouteSlug(metadataEntities.value, String(route.params.entity ?? '')))
const recordEntities = computed(() => searchableEntities.value.filter((entity) => !entity['is-single']))
const selectedSearchEntity = computed(() => recordEntities.value.find((entity) => entity.slug === searchEntity.value))
const recordResults = useQuery({
  queryKey: computed(() => ['command-records', authStore.currentUser?.id, authStore.sessionVersion, searchEntity.value, recordSearch.value]),
  enabled: computed(() => commandMenuOpen.value && searchingRecords.value && Boolean(authStore.currentUser && selectedSearchEntity.value && recordSearch.value)),
  queryFn: ({ signal }) => listRecords(searchEntity.value, {
    limit: 20,
    offset: 0,
    filters: [{ field: 'name', operator: 'contains', value: recordSearch.value }],
  }, { signal }),
})

watch([query, searchEntity, searchingRecords, commandMenuOpen], ([value], _previous, cleanup) => {
  recordSearch.value = ''
  if (!commandMenuOpen.value || !searchingRecords.value) return
  const timer = setTimeout(() => { recordSearch.value = value.trim() }, 250)
  cleanup(() => clearTimeout(timer))
})

const contextualItems = computed<CommandItem[]>(() => [
  ...pageCommands.value.map((command) => ({ ...command, detail: command.detail ?? '', keywords: command.keywords ?? '', icon: Command })),
])

watch(() => navigationStore.recordSearchRequested, requested => {
  if (!requested) return
  navigationStore.recordSearchRequested = false
  startRecordSearch()
})

function startRecordSearch() {
  searchingRecords.value = true
  searchEntity.value = currentEntity.value && !currentEntity.value['is-single'] ? currentEntity.value.slug ?? '' : ''
  query.value = ''
  searchInput.value?.focus()
}

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
  ...globalCommands.value.filter(command => command.id !== 'app:palette').map(command => ({ ...commandBinding(command), detail: '', keywords: '', icon: Command })),
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
  if (searchingRecords.value) {
    const entity = selectedSearchEntity.value
    if (!entity || query.value.trim() !== recordSearch.value) return []
    return [{ key: 'records', label: entityLabel(entity), items: (recordResults.data.value?.data ?? []).flatMap((record): CommandItem[] => (
      typeof record.name === 'string' ? [{
        id: `record:${entity.slug}:${record.name}`, label: record.name,
        detail: [record['full-name'], record.title, record.label].find((value): value is string => typeof value === 'string' && value.length > 0) ?? entityLabel(entity),
        keywords: record.name, icon: iconForEntity(entity.icon),
        run: () => router.push({ name: RouteName.RecordDetail, params: { entity: entity.slug!, recordName: record.name as string } }).then(() => {}),
      }] : []
    )) }].filter((group) => group.items.length > 0)
  }
  if (!hasQuery.value) {
    return [
      { key: 'context', label: 'This page', items: contextualItems.value },
      { key: 'recent', label: 'Recent', items: recentCommandItems.value },
      { key: 'app-actions', label: 'Studio', items: appActionItems.value },
    ].filter((group) => group.items.length > 0)
  }

  return [
    { key: 'context', label: 'This page', items: filterItems(contextualItems.value, normalizedQuery.value) },
    { key: 'pages', label: 'Pages', items: filterItems(pageCommandItems.value, normalizedQuery.value) },
    { key: 'create', label: 'Create', items: filterItems(createCommandItems.value, normalizedQuery.value) },
    { key: 'app-actions', label: 'App actions', items: filterItems(appActionItems.value, normalizedQuery.value) },
  ].filter((group) => group.items.length > 0)
})

const visibleItems = computed(() => commandGroups.value.flatMap((group) => group.items))
const emptyMessage = computed(() => {
  if (!searchingRecords.value) return hasQuery.value ? 'No matching commands' : 'No recent pages yet'
  if (!selectedSearchEntity.value) return 'Select an Entity'
  if (!query.value.trim()) return 'Type a Record ID'
  if (recordSearch.value !== query.value.trim() || recordResults.isFetching.value) return 'Searching…'
  if (recordResults.error.value) return 'Could not search Records. Try again.'
  return 'No matching Records'
})
const activeDescendantId = computed(() => {
  const activeItem = visibleItems.value.find((item) => item.id === activeItemId.value)
  return activeItem ? itemDomId(activeItem) : undefined
})

const currentRecentPage = computed(() => recentPageForRoute(route))

watch(activeDescendantId, async (id) => {
  await nextTick()
  if (id) document.getElementById(id)?.scrollIntoView({ block: 'nearest' })
})

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
    const activeItem = items.find((item) => item.id === activeItemId.value)
    if (activeItem && !unavailable(activeItem)) {
      return
    }

    activeItemId.value = items.find((item) => !unavailable(item))?.id ?? ''
  },
  { immediate: true },
)

watch(
  commandMenuOpen,
  async (open) => {
    if (!open) {
      query.value = ''
      searchingRecords.value = false
      recordSearch.value = ''
      return
    }

    returnFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
    await nextTick()
    searchInput.value?.focus()
  },
)

function closeMenu() {
  navigationStore.closeCommandMenu()
}

function updateMenuOpen(open: boolean) {
  if (open) {
    navigationStore.openCommandMenu()
  } else {
    closeMenu()
  }
}

function handleInputKeydown(event: KeyboardEvent) {
  if (event.isComposing || event.repeat || event.defaultPrevented) return
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
  activeItemId.value = nextAvailableCommandId(visibleItems.value, activeItemId.value, direction)
}

async function runCommand(item: CommandItem) {
  const current = studioCommands.value.find(command => command.id === item.id) ?? item
  if (unavailable(current)) return
  if (item.id === 'records:search') { await executeCommand(current); return }
  afterClose = studioCommands.value.some(command => command.id === item.id)
    ? () => runStudioCommand(item.id)
    : () => executeCommand(item)
  closeMenu()
}

async function restoreFocus(event: Event) {
  event.preventDefault()
  const action = afterClose
  afterClose = null
  const target = returnFocus
  returnFocus = null
  // In-place commands and dialogs need a stable return target.
  target?.focus()
  if (!action) return
  await nextTick()
  const previousPath = route.fullPath
  await action()
  // Successful navigation should not leave a focus ring on Search. Keep
  // focus for cancelled navigation and let explicit focus commands own it.
  if (route.fullPath !== previousPath && target === triggerButton.value && document.activeElement === target) {
    target?.blur()
  }
}

async function navigateTo(to: string | { name: string, params?: Record<string, string> }) {
  await router.push(to)
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

function entityLabel(entity: MetadataEntity): string {
  return entity.label || humanizeEntity(entity.slug ?? entity.key)
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
    <Dialog
      :open="commandMenuOpen"
      title="Command menu"
      title-sr-only
      description="Search pages, actions, and Records."
      description-class="sr-only"
      :panel-attrs="{ 'data-studio-command-menu': true }"
      :panel-class="searchingRecords ? 'studio-command-menu__dialog studio-command-menu__dialog--palette studio-command-menu__dialog--records' : 'studio-command-menu__dialog studio-command-menu__dialog--palette'"
      overlay-class="studio-command-menu__overlay"
      @update:open="updateMenuOpen"
      @close-auto-focus="restoreFocus"
    >
      <template #trigger>
        <button
          ref="triggerButton"
          class="studio-command-menu__trigger"
          type="button"
          aria-haspopup="dialog"
          :aria-expanded="commandMenuOpen"
          :aria-keyshortcuts="ariaShortcut(paletteShortcut)"
          :title="`Search (${shortcutLabel(paletteShortcut)})`"
        >
          <Search class="studio-command-menu__search-icon" :size="14" :stroke-width="1.8" aria-hidden="true" />
          <span class="studio-command-menu__placeholder">Search</span>
          <span class="studio-command-menu__shortcut" aria-hidden="true">
            <kbd>{{ shortcutLabel(paletteShortcut) }}</kbd>
          </span>
          <span class="studio-command-menu__shortcut-label">{{ shortcutLabel(paletteShortcut) }}</span>
        </button>
      </template>
      <div v-if="searchingRecords" class="studio-command-menu__input-wrap">
        <button class="studio-command-menu__back" type="button" aria-label="Back to commands" @click="searchingRecords = false; query = ''; searchInput?.focus()">
          <ArrowLeft :size="16" aria-hidden="true" />
          Back
        </button>
        <select v-model="searchEntity" aria-label="Search Entity" class="studio-command-menu__entity-select">
          <option value="">Select an Entity</option>
          <option v-for="entity in recordEntities" :key="entity.name" :value="entity.slug">{{ entityLabel(entity) }}</option>
        </select>
      </div>
      <div class="studio-command-menu__input-wrap">
        <Search class="studio-command-menu__dialog-icon" :size="16" :stroke-width="1.8" aria-hidden="true" />
        <input
          ref="searchInput"
          v-model="query"
          class="studio-command-menu__input"
          type="search"
          :placeholder="searchingRecords ? 'Search Record IDs' : 'Search or type a command'"
          :aria-label="searchingRecords ? 'Search Record IDs' : 'Search commands'"
          aria-autocomplete="list"
          autocomplete="off"
          spellcheck="false"
          role="combobox"
          aria-controls="studio-command-menu-list"
          :aria-expanded="commandMenuOpen"
          :aria-activedescendant="activeDescendantId"
          @keydown="handleInputKeydown"
        >
        <span class="studio-command-menu__dialog-shortcut" aria-hidden="true">
          <kbd>{{ shortcutLabel(paletteShortcut) }}</kbd>
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
              :disabled="unavailable(item)"
              tabindex="-1"
              @mouseenter="!unavailable(item) && (activeItemId = item.id)"
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
                <span v-if="item.disabledReason || item.detail" class="studio-command-menu__item-detail">{{ item.disabledReason || item.detail }}</span>
              </span>
              <kbd v-if="commandBinding(item).shortcut" class="studio-command-menu__item-shortcut">{{ shortcutLabel(commandBinding(item).shortcut) }}</kbd>
            </button>
          </section>
        </template>

        <div v-else class="studio-command-menu__empty" role="status">
          {{ emptyMessage }}
        </div>
      </div>

      <div class="studio-command-menu__footer" aria-hidden="true">
        <span class="studio-command-menu__hint">
          <kbd>↑</kbd>
          <kbd>↓</kbd>
          <span>navigate</span>
        </span>
        <span class="studio-command-menu__hint">
          <kbd>Enter</kbd>
          <span>select</span>
        </span>
        <span class="studio-command-menu__hint">
          <kbd>Esc</kbd>
          <span>close</span>
        </span>
      </div>
    </Dialog>
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
  background: var(--studio-overlay);
}

.studio-command-menu__dialog {
  display: grid;
  width: min(640px, calc(100vw - 32px));
  max-height: min(620px, calc(100vh - 120px));
  overflow: hidden;
  border: 1px solid var(--studio-border);
  border-radius: var(--studio-radius-panel);
  background: var(--studio-surface);
  box-shadow: var(--studio-shadow-sheet);
  position: fixed;
  z-index: 61;
  top: 86px;
  left: 50%;
  transform: translateX(-50%);
}

.studio-command-menu__dialog--palette {
  grid-template-rows: auto minmax(0, 1fr) auto;
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
  min-height: 0;
  overscroll-behavior: contain;
  scrollbar-gutter: stable;
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
  min-height: 48px;
  grid-template-columns: 22px minmax(0, 1fr) auto;
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

.studio-command-menu__item:not(:disabled):hover,
.studio-command-menu__item:focus-visible,
.studio-command-menu__item--active:not(:disabled) {
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
  flex-direction: column;
  align-items: stretch;
  gap: 3px;
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
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--studio-text-subtle);
  font-size: 12px;
  font-weight: 400;
  line-height: 1.4;
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
    top: 16px;
    max-height: calc(100dvh - 32px);
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

.studio-command-menu__dialog--records {
  grid-template-rows: auto auto minmax(0, 1fr) auto;
}

.studio-command-menu__back {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border: 0;
  border-radius: var(--studio-radius-control);
  padding: 6px 8px;
  background: transparent;
  color: var(--studio-text-muted);
  font: inherit;
  font-size: 13px;
  cursor: pointer;
}

.studio-command-menu__back:hover {
  background: var(--studio-surface-raised);
  color: var(--studio-text);
}

.studio-command-menu__back:focus-visible,
.studio-command-menu__entity-select:focus-visible {
  outline: 2px solid var(--studio-focus);
  outline-offset: 2px;
}

.studio-command-menu__entity-select {
  flex: 1;
  min-width: 0;
  border: 1px solid var(--studio-border);
  border-radius: var(--studio-radius-control);
  background: var(--studio-control-bg);
  color: var(--studio-text);
  padding: 6px 8px;
  font: inherit;
  font-size: 13px;
}

.studio-command-menu__item-shortcut {
  border: 1px solid var(--studio-border);
  border-radius: 5px;
  background: var(--studio-neutral-soft);
  color: var(--studio-text-subtle);
  padding: 3px 5px;
  font: inherit;
  font-size: 11px;
  white-space: nowrap;
}

.studio-command-menu__item--active:not(:disabled) {
  box-shadow: inset 2px 0 var(--studio-focus);
}
</style>
