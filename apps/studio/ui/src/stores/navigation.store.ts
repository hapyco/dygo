import { defineStore } from 'pinia'
import { useStorage } from '@vueuse/core'

export type RecentPage = {
  path: string
  label: string
  detail: string
}

const RECENT_PAGES_STORAGE_KEY = 'dygo.studio.recentPages'
const MAX_RECENT_PAGES = 10
// TODO(preferences): move recent pages into the Preference entity once per-user UI state lands.
const recentPagesStorage = useStorage<RecentPage[]>(RECENT_PAGES_STORAGE_KEY, [], undefined, {
  onError: () => {},
})

export const useNavigationStore = defineStore('navigation', {
  state: () => ({
    sidebarCollapsed: false,
    commandMenuOpen: false,
    routeReloadVersion: 0,
    recentPages: normalizeRecentPages(recentPagesStorage.value),
  }),

  actions: {
    setSidebarCollapsed(value: boolean) {
      this.sidebarCollapsed = value
    },

    toggleSidebar() {
      this.sidebarCollapsed = !this.sidebarCollapsed
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

    rememberRecentPage(page: RecentPage | null) {
      if (!page || page.path.trim() === '' || page.label.trim() === '') {
        return
      }

      this.recentPages = [
        page,
        ...this.recentPages.filter((recentPage) => recentPage.path !== page.path),
      ].slice(0, MAX_RECENT_PAGES)

      recentPagesStorage.value = this.recentPages
    },
  },
})

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
