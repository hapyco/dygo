<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { X } from '@lucide/vue'

import { studioPathsEqual } from '@/router/current'
import { useNavigationStore } from '@/stores/navigation.store'
import { useCloseTab } from './use-close-tab'

const route = useRoute()
const router = useRouter()
const navigation = useNavigationStore()
const closeTab = useCloseTab()
const tabs = computed(() => navigation.openTabs)
const canClose = computed(() => tabs.value.length > 1)

async function activate(path: string) {
  const tab = tabs.value.find(item => item.path === path)
  if (!tab || studioPathsEqual(tab.path, route.path)) return
  await router.push(tab.fullPath)
}

function tabIsActive(path: string) {
  return studioPathsEqual(path, route.path)
}

function onTabKeydown(event: KeyboardEvent, path: string) {
  if (event.key === 'ArrowRight' || event.key === 'ArrowLeft') {
    event.preventDefault()
    const tab = navigation.cycleTab(path, event.key === 'ArrowRight' ? 1 : -1)
    if (!tab) return
    const target = document.getElementById(tabId(tab.path))
    target?.focus()
    void activate(tab.path)
  } else if ((event.key === 'Delete' || event.key === 'Backspace') && canClose.value) {
    event.preventDefault()
    void closeTab(path)
  }
}

function tabId(path: string) {
  return `studio-tab-${encodeURIComponent(path)}`
}
</script>

<template>
  <div v-if="tabs.length > 0" class="page-tabs" role="tablist" aria-label="Open pages">
    <div
      v-for="tab in tabs"
      :key="tab.path"
      class="page-tabs__item"
      :class="{ 'page-tabs__item--active': tabIsActive(tab.path), 'page-tabs__item--dirty': navigation.dirtyTabPaths.includes(tab.path) }"
    >
      <button
        :id="tabId(tab.path)"
        class="page-tabs__tab"
        :class="{ 'page-tabs__tab--active': tabIsActive(tab.path) }"
        type="button"
        role="tab"
        :aria-selected="tabIsActive(tab.path)"
        :tabindex="tabIsActive(tab.path) ? 0 : -1"
        :title="tab.label"
        @click="activate(tab.path)"
        @keydown="onTabKeydown($event, tab.path)"
        @auxclick.prevent="canClose && $event.button === 1 && closeTab(tab.path)"
      >
        <span class="page-tabs__label">{{ tab.label }}</span>
        <span v-if="navigation.dirtyTabPaths.includes(tab.path)" class="page-tabs__dirty" aria-label="Unsaved changes" />
      </button>
      <button
        v-if="canClose"
        class="page-tabs__close"
        type="button"
        :aria-label="`Close ${tab.label}`"
        @click.stop="closeTab(tab.path)"
      >
        <X :size="12" :stroke-width="2.2" aria-hidden="true" />
      </button>
    </div>
  </div>
</template>

<style scoped>
.page-tabs {
  display: flex;
  min-width: 0;
  align-items: end;
  gap: 4px;
  overflow-x: auto;
  padding: 8px 2px 0;
}

.page-tabs__item {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  min-width: 88px;
  max-width: 220px;
  min-height: 32px;
  gap: 4px;
  border: 1px solid transparent;
  border-bottom: 0;
  border-radius: var(--studio-radius-sheet) var(--studio-radius-sheet) 0 0;
  background: transparent;
  color: var(--studio-text-muted);
  padding: 0 6px 0 12px;
}

.page-tabs__item:hover {
  background: var(--studio-surface-raised);
  color: var(--studio-text);
}

.page-tabs__item--active {
  background: var(--studio-surface);
  border-color: var(--studio-border);
  color: var(--studio-text);
  box-shadow: 0 1px 0 var(--studio-surface);
}

.page-tabs__tab {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  min-width: 0;
  min-height: 32px;
  border: 0;
  background: transparent;
  color: inherit;
  padding: 0;
  cursor: pointer;
  text-align: left;
}

.page-tabs__tab:focus-visible,
.page-tabs__close:focus-visible {
  outline: 2px solid var(--studio-focus);
  outline-offset: -2px;
}

.page-tabs__label {
  display: block;
  min-width: 0;
  flex: 1 1 auto;
  overflow: hidden;
  font-size: 12px;
  font-weight: 600;
  line-height: 1.2;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.page-tabs__dirty {
  content: '';
  display: inline-block;
  flex: 0 0 auto;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--studio-warning);
  vertical-align: middle;
}

.page-tabs__close {
  display: inline-flex;
  width: 22px;
  height: 22px;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: var(--studio-radius-control);
  background: transparent;
  color: inherit;
}

.page-tabs__close:hover {
  background: var(--studio-surface-raised);
  color: var(--studio-text);
}

@media (max-width: 720px) {
  .page-tabs {
    padding: 6px 0 0;
  }

  .page-tabs__tab {
    min-width: 72px;
  }
}
</style>
