<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import {
  ChartNoAxesColumn,
  Database,
  Home,
  Settings,
  Workflow,
} from '@lucide/vue'

import { loadCurrentUser } from '@/features/auth/session'
import type { CurrentUser } from '@/features/auth/auth.api'
import { RouteName } from '@/router/routes'
import Shell from '@/shell/Shell.vue'
import type { ShellNavItem } from '@/shell/types'

const route = useRoute()
const currentUser = ref<CurrentUser | null>(null)

const usesShell = computed(() => !route.meta.public)

const navItems = computed<ShellNavItem[]>(() => [
  {
    label: 'Home',
    to: '/',
    icon: Home,
    current: route.name === RouteName.Home,
  },
  {
    label: 'Records',
    to: '/records',
    icon: Database,
    current: route.path.startsWith('/records'),
  },
  {
    label: 'Reports',
    to: '/reports',
    icon: ChartNoAxesColumn,
    current: route.path.startsWith('/reports'),
  },
  {
    label: 'Workflows',
    to: '/workflows',
    icon: Workflow,
    current: route.path.startsWith('/workflows'),
  },
  {
    label: 'Settings',
    to: '/settings',
    icon: Settings,
    current: route.path.startsWith('/settings'),
  },
])

const userName = computed(() => currentUser.value?.['full-name'] || currentUser.value?.email || 'Studio user')

watch(
  usesShell,
  async (enabled) => {
    if (!enabled) {
      currentUser.value = null
      return
    }

    currentUser.value = await loadCurrentUser()
  },
  { immediate: true },
)
</script>

<template>
  <RouterView v-if="!usesShell" />
  <Shell v-else :user-name="userName" :nav-items="navItems">
    <RouterView />
  </Shell>
</template>
