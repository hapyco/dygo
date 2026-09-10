import { defineStore } from 'pinia'
import { usePreferencesStore } from '../features/preferences/preferences.store.ts'
import { normalizePinnedItems, pinnedItemID, type PinnedItem } from '../features/pinned/pinned.ts'
import {
  closeStudioTab,
  cycleStudioTab,
  morphStudioTab,
  normalizeStudioTabs,
  shouldMorphNewRecordTab,
  tabCacheKey,
  upsertStudioTab,
  type StudioTab,
} from '../features/tabs/tabs.ts'

export type RecentPage = {
  path: string
  label: string
  detail: string
}

const RECENT_PAGES_STORAGE_KEY = 'dygo.studio.recentPages'
const OPEN_TABS_STORAGE_KEY = 'dygo.studio.openTabs'
const MAX_RECENT_PAGES = 10

export const useNavigationStore = defineStore('navigation', {
  state: () => ({
    recentUserID: null as number | null,
    recentGeneration: 0,
    commandMenuOpen: false,
    shortcutsOpen: false,
    recordSearchRequested: false,
    routeReloadVersion: 0,
    dirtyTabPaths: [] as string[],
    openTabs: [] as StudioTab[],
  }),

  getters: {
    sidebarCollapsed: () => usePreferencesStore().get<boolean>('studio.sidebar-collapsed', false) === true,
    recentPages: () => normalizeRecentPages(usePreferencesStore().get('studio.recent-pages', [])),
    pinnedItems: () => normalizePinnedItems(usePreferencesStore().get('studio.pinned-items', [])),
    pinnedOpen: () => usePreferencesStore().get<boolean>('studio.pinned-open', true) !== false,
    pinnedExpanded: () => usePreferencesStore().get<boolean>('studio.pinned-expanded', false) === true,
  },

  actions: {
    setRecentUser(userID: number | null) {
      this.recentGeneration++
      this.recentUserID = userID
      this.commandMenuOpen = false
      this.shortcutsOpen = false
      this.recordSearchRequested = false
      this.dirtyTabPaths = []
      this.openTabs = userID === null ? [] : readOpenTabs(userID)
      const preferences = usePreferencesStore()
      void preferences.startSession(userID)
      if (userID !== null) void preferences.importMissing({ 'studio.recent-pages': readRecentPages(userID) })
    },

    clearRecentPages() {
      usePreferencesStore().set('studio.recent-pages', [])
    },

    setSidebarCollapsed(value: boolean) {
      usePreferencesStore().set('studio.sidebar-collapsed', value)
    },

    toggleSidebar() {
      this.setSidebarCollapsed(!this.sidebarCollapsed)
    },

    pin(item: PinnedItem) {
      const id = pinnedItemID(item)
      usePreferencesStore().set('studio.pinned-items', [item, ...this.pinnedItems.filter(candidate => pinnedItemID(candidate) !== id)])
    },

    unpin(item: PinnedItem) {
      const id = pinnedItemID(item)
      usePreferencesStore().set('studio.pinned-items', this.pinnedItems.filter(candidate => pinnedItemID(candidate) !== id))
    },

    togglePin(item: PinnedItem) {
      if (this.pinnedItems.some(candidate => pinnedItemID(candidate) === pinnedItemID(item))) this.unpin(item)
      else this.pin(item)
    },

    reorderPinned(from: number, to: number) {
      if (from === to || from < 0 || to < 0 || from >= this.pinnedItems.length || to >= this.pinnedItems.length) return
      const items = [...this.pinnedItems]
      const [item] = items.splice(from, 1)
      if (!item) return
      items.splice(to, 0, item)
      usePreferencesStore().set('studio.pinned-items', items)
    },

    setPinnedOpen(value: boolean) {
      usePreferencesStore().set('studio.pinned-open', value)
    },

    setPinnedExpanded(value: boolean) {
      usePreferencesStore().set('studio.pinned-expanded', value)
    },

    openCommandMenu() {
      this.commandMenuOpen = true
    },

    closeCommandMenu() {
      this.commandMenuOpen = false
    },

    requestRouteReload() {
      this.routeReloadVersion += 1
    },

    tabCacheKey(path: string) {
      return tabCacheKey(this.openTabs, path, this.routeReloadVersion)
    },

    isTabDirty(path: string) {
      return this.dirtyTabPaths.includes(path)
    },

    setTabDirty(path: string, dirty: boolean) {
      if (dirty) {
        if (!this.dirtyTabPaths.includes(path)) this.dirtyTabPaths = [...this.dirtyTabPaths, path]
        return
      }
      this.dirtyTabPaths = this.dirtyTabPaths.filter(item => item !== path)
    },

    setTabLabel(path: string, label: string) {
      const nextLabel = label.trim()
      if (!nextLabel) return
      this.writeTabs(this.openTabs.map(tab => tab.path === path ? { ...tab, label: nextLabel } : tab))
    },

    syncTab(tab: StudioTab, previousPath = '') {
      if (this.recentUserID === null) return
      if (shouldMorphNewRecordTab(previousPath, tab.path)) {
        this.replaceTab(previousPath, tab)
        return
      }
      this.writeTabs(upsertStudioTab(this.openTabs, tab))
    },

    replaceTab(fromPath: string, tab: StudioTab) {
      this.setTabDirty(fromPath, false)
      this.writeTabs(morphStudioTab(this.openTabs, fromPath, tab))
    },

    closeTab(path: string, activePath: string) {
      const result = closeStudioTab(this.openTabs, path, activePath)
      if (!result.closed) return result.activate
      this.setTabDirty(path, false)
      this.writeTabs(result.tabs)
      return result.activate
    },

    cycleTab(activePath: string, delta: number) {
      return cycleStudioTab(this.openTabs, activePath, delta)
    },

    writeTabs(tabs: StudioTab[]) {
      this.openTabs = normalizeStudioTabs(tabs)
      if (this.recentUserID !== null) persistOpenTabs(this.recentUserID, this.openTabs)
    },

    async rememberRecentPage(page: RecentPage | null) {
      if (!page || page.path.trim() === '' || page.label.trim() === '') {
        return
      }

      const generation = this.recentGeneration
      const preferences = usePreferencesStore()
      await preferences.startSession(this.recentUserID)
      if (generation !== this.recentGeneration || !preferences.ready) return

      const pages = [
        page,
        ...this.recentPages.filter((recentPage) => recentPage.path !== page.path),
      ].slice(0, MAX_RECENT_PAGES)

      usePreferencesStore().set('studio.recent-pages', pages)
    },
  },
})

function recentPagesKey(userID: number): string {
  return `${RECENT_PAGES_STORAGE_KEY}.${userID}`
}

function openTabsKey(userID: number): string {
  return `${OPEN_TABS_STORAGE_KEY}.${userID}`
}

function readOpenTabs(userID: number): StudioTab[] {
  try {
    return normalizeStudioTabs(JSON.parse(window.sessionStorage.getItem(openTabsKey(userID)) ?? '[]'))
  } catch {
    return []
  }
}

function persistOpenTabs(userID: number, tabs: StudioTab[]) {
  try {
    window.sessionStorage.setItem(openTabsKey(userID), JSON.stringify(tabs))
  } catch {
    // Browser session storage is optional.
  }
}

function readRecentPages(userID: number): RecentPage[] {
  try {
    return normalizeRecentPages(JSON.parse(window.localStorage.getItem(recentPagesKey(userID)) ?? '[]'))
  } catch {
    return []
  }
}

function normalizeRecentPages(value: unknown): RecentPage[] {
  if (!Array.isArray(value)) {
    return []
  }

  return value
    .map((item): RecentPage | null => {
      if (!item || typeof item !== 'object') {
        return null
      }

      const path = typeof item.path === 'string' ? item.path : ''
      const label = typeof item.label === 'string' ? item.label : ''
      const detail = typeof item.detail === 'string' ? item.detail : ''
      if (!path || !label) {
        return null
      }

      return { path, label, detail }
    })
    .filter((item): item is RecentPage => Boolean(item))
    .slice(0, MAX_RECENT_PAGES)
}
