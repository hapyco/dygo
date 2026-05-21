import { defineStore } from 'pinia'

export const useNavigationStore = defineStore('navigation', {
  state: () => ({
    sidebarCollapsed: false,
    commandMenuOpen: false,
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
  },
})
