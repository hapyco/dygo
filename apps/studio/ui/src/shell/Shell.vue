<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { computed, onScopeDispose, watchEffect } from 'vue'
import { useMediaQuery } from '@vueuse/core'
import { useRouter } from 'vue-router'
import { useBootStore } from '@/stores/boot.store'
import { useAuthStore } from '@/stores/auth.store'
import { globalCommands, runStudioCommand } from '@/features/commands/context'
import { useShortcuts } from '@/features/commands/use-shortcuts'
import KeyboardShortcuts from './KeyboardShortcuts.vue'

import { useNavigationStore } from '@/stores/navigation.store'
import type { ShellNavItem } from './types'
import PageSheet from './PageSheet.vue'
import Sidebar from './Sidebar.vue'
import TopBar from './TopBar.vue'
import PageTabs from '@/features/tabs/PageTabs.vue'
import { useCloseTab } from '@/features/tabs/use-close-tab'

const props = withDefaults(defineProps<{
  brandLabel?: string
  brandMark?: string
  userName?: string
  userAvatarUrl?: string
  navItems?: ShellNavItem[]
  showSidebar?: boolean
}>(), {
  brandLabel: 'dygo Studio',
  brandMark: 'd',
  userName: 'Studio user',
  navItems: () => [],
  showSidebar: true,
})

const navigationStore = useNavigationStore()
const { sidebarCollapsed, openTabs } = storeToRefs(navigationStore)
const router = useRouter()
const boot = useBootStore()
const auth = useAuthStore()
const desktop = useMediaQuery('(min-width: 721px)')
const closeTab = useCloseTab()
const commands = computed(() => auth.currentUser ? [
  { id: 'app:palette', label: 'Open command palette', group: 'Studio', run: () => { navigationStore.commandMenuOpen = !navigationStore.commandMenuOpen } },
  { id: 'app:shortcuts', label: 'Keyboard shortcuts', group: 'Studio', run: () => { navigationStore.shortcutsOpen = true } },
  { id: 'records:search', label: 'Search Records', group: 'Studio', run: () => { navigationStore.openCommandMenu(); navigationStore.recordSearchRequested = true } },
  { id: 'app:sidebar', label: 'Toggle sidebar', group: 'Studio', disabledReason: props.showSidebar && desktop.value ? undefined : 'Sidebar collapse is unavailable', run: () => navigationStore.toggleSidebar() },
  { id: 'app:home', label: 'Go home', group: 'Studio', run: async () => {
    const home = boot.defaults?.home
    await router.push(typeof home === 'string' && home.startsWith('/') && !home.startsWith('//') ? home : '/')
  } },
  { id: 'app:tab-close', label: 'Close tab', group: 'Studio', disabledReason: openTabs.value.length <= 1 ? 'The last tab stays open' : undefined, run: () => closeTab(router.currentRoute.value.path) },
  { id: 'app:tab-next', label: 'Next tab', group: 'Studio', disabledReason: openTabs.value.length < 2 ? 'No other tab is open' : undefined, run: async () => {
    const tab = navigationStore.cycleTab(router.currentRoute.value.path, 1)
    if (tab) await router.push(tab.fullPath)
  } },
  { id: 'app:tab-previous', label: 'Previous tab', group: 'Studio', disabledReason: openTabs.value.length < 2 ? 'No other tab is open' : undefined, run: async () => {
    const tab = navigationStore.cycleTab(router.currentRoute.value.path, -1)
    if (tab) await router.push(tab.fullPath)
  } },
] : [])
watchEffect(() => { globalCommands.value = commands.value })
onScopeDispose(() => { globalCommands.value = [] })
useShortcuts()
</script>

<template>
  <div class="studio-shell" :class="{ 'studio-shell--sidebar-collapsed': sidebarCollapsed && showSidebar, 'studio-shell--no-sidebar': !showSidebar }">
    <KeyboardShortcuts />
    <TopBar
      class="studio-shell__header"
      :brand-label="brandLabel"
      :brand-mark="brandMark"
      :user-name="userName"
      :user-avatar-url="userAvatarUrl"
    >
      <template #actions>
        <slot name="header-actions" />
      </template>
    </TopBar>

    <Sidebar
      v-if="showSidebar"
      :collapsed="sidebarCollapsed"
      @update:collapsed="runStudioCommand('app:sidebar')"
      class="studio-shell__sidebar"
      :items="navItems"
    >
      <slot name="sidebar" />
    </Sidebar>

    <div class="studio-shell__sheet">
      <PageTabs />
      <PageSheet>
        <slot />
      </PageSheet>
    </div>
  </div>
</template>

<style scoped>
.studio-shell {
  display: grid;
  height: 100vh;
  min-height: 0;
  grid-template-columns: var(--studio-shell-sidebar-width) minmax(0, 1fr);
  grid-template-rows: var(--studio-shell-header-height) minmax(0, 1fr);
  overflow: hidden;
  background: var(--studio-bg);
  background-image: none;
}

.studio-shell--sidebar-collapsed {
  grid-template-columns: 64px minmax(0, 1fr);
}

.studio-shell--no-sidebar {
  grid-template-columns: minmax(0, 1fr);
}

.studio-shell--no-sidebar .studio-shell__sheet {
  grid-column: 1;
  padding-left: var(--studio-shell-sheet-right-gutter);
}

.studio-shell__header {
  grid-column: 1 / -1;
  grid-row: 1;
}

.studio-shell__sidebar {
  grid-column: 1;
  grid-row: 2;
  min-height: 0;
  overflow: hidden;
}

.studio-shell__sheet {
  position: relative;
  z-index: 1;
  display: grid;
  min-height: 0;
  min-width: 0;
  grid-column: 2;
  grid-row: 2;
  grid-template-rows: auto minmax(0, 1fr);
  overflow: visible;
  padding: 0 var(--studio-shell-sheet-right-gutter) 0 0;
}

@media (max-width: 720px) {
  .studio-shell {
    grid-template-columns: minmax(0, 1fr);
    grid-template-rows: var(--studio-shell-header-height) auto minmax(0, 1fr);
  }

  .studio-shell__header {
    grid-column: 1;
    grid-row: 1;
  }

  .studio-shell__sidebar {
    grid-column: 1;
    grid-row: 2;
  }

  .studio-shell__sheet {
    grid-column: 1;
    grid-row: 3;
    padding: 0 12px 0;
  }
}
</style>
