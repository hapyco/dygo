import { defineStore } from 'pinia'

export const useNavigationStore = defineStore('navigation', {
  state: () => ({
    sidebarCollapsed: false,
    commandMenuOpen: false,
    routeReloadVersion: 0,
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
  },
})
