<script setup lang="ts">
import { storeToRefs } from 'pinia'

import { useNavigationStore } from '@/stores/navigation.store'
import type { ShellNavItem } from './types'
import PageSheet from './PageSheet.vue'
import Sidebar from './Sidebar.vue'
import TopBar from './TopBar.vue'

withDefaults(defineProps<{
  brandLabel?: string
  brandMark?: string
  companyName?: string
  userName?: string
  userAvatarUrl?: string
  navItems?: ShellNavItem[]
}>(), {
  brandLabel: 'dygo Studio',
  brandMark: 'd',
  companyName: 'Umami Smokehouse',
  userName: 'Studio user',
  navItems: () => [],
})

const navigationStore = useNavigationStore()
const { sidebarCollapsed } = storeToRefs(navigationStore)
</script>

<template>
  <div class="studio-shell" :class="{ 'studio-shell--sidebar-collapsed': sidebarCollapsed }">
    <TopBar
      class="studio-shell__header"
      :brand-label="brandLabel"
      :brand-mark="brandMark"
      :company-name="companyName"
      :user-name="userName"
      :user-avatar-url="userAvatarUrl"
    >
      <template #actions>
        <slot name="header-actions" />
      </template>
    </TopBar>

    <Sidebar
      v-model:collapsed="sidebarCollapsed"
      class="studio-shell__sidebar"
      :items="navItems"
    >
      <slot name="sidebar" />
    </Sidebar>

    <div class="studio-shell__sheet">
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
