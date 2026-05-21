<script setup lang="ts">
import { PanelLeftClose, PanelLeftOpen } from '@lucide/vue'
import { RouterLink } from 'vue-router'

import type { ShellNavItem } from './types'

withDefaults(defineProps<{
  ariaLabel?: string
  collapsed?: boolean
  items?: ShellNavItem[]
}>(), {
  ariaLabel: 'Studio navigation',
  collapsed: false,
  items: () => [],
})

const emit = defineEmits<{
  'update:collapsed': [collapsed: boolean]
}>()
</script>

<template>
  <aside class="studio-sidebar" :class="{ 'studio-sidebar--collapsed': collapsed }" :aria-label="ariaLabel">
    <nav v-if="items.length > 0" class="studio-sidebar__nav">
      <RouterLink
        v-for="item in items"
        :key="item.label"
        class="studio-sidebar__item"
        :class="{ 'studio-sidebar__item--current': item.current }"
        :to="item.to"
        :aria-current="item.current ? 'page' : undefined"
        :title="collapsed ? item.label : undefined"
      >
        <component
          :is="item.icon"
          v-if="item.icon"
          class="studio-sidebar__item-icon"
          aria-hidden="true"
          :size="16"
          :stroke-width="1.8"
        />
        <span class="studio-sidebar__item-label">{{ item.label }}</span>
      </RouterLink>
    </nav>
    <slot />

    <button
      class="studio-sidebar__collapse"
      type="button"
      :aria-label="collapsed ? 'Expand sidebar' : 'Collapse sidebar'"
      :title="collapsed ? 'Expand sidebar' : 'Collapse sidebar'"
      @click="emit('update:collapsed', !collapsed)"
    >
      <PanelLeftOpen v-if="collapsed" aria-hidden="true" :size="16" :stroke-width="1.8" />
      <PanelLeftClose v-else aria-hidden="true" :size="16" :stroke-width="1.8" />
    </button>
  </aside>
</template>

<style scoped>
.studio-sidebar {
  display: flex;
  min-height: 0;
  min-width: 0;
  flex-direction: column;
  overflow: hidden;
  padding: 18px 14px 24px var(--studio-shell-gutter);
  transition: padding 160ms ease;
}

.studio-sidebar--collapsed {
  align-items: center;
  padding: 18px 10px 24px;
}

.studio-sidebar__nav {
  display: grid;
  flex: 1 1 auto;
  min-height: 0;
  align-content: start;
  gap: 4px;
  grid-auto-rows: min-content;
  overflow-y: auto;
  width: 100%;
}

.studio-sidebar__item {
  display: flex;
  min-height: 34px;
  align-items: center;
  gap: 9px;
  border-radius: var(--studio-radius-control);
  color: var(--studio-text-muted);
  font-size: 13px;
  font-weight: 600;
  line-height: 1;
  padding: 0 10px;
  text-decoration: none;
  transition:
    background-color 160ms ease,
    color 160ms ease;
}

.studio-sidebar__item-icon {
  flex: 0 0 auto;
}

.studio-sidebar__item-label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.studio-sidebar__item:hover {
  background: var(--studio-surface-raised);
  color: var(--studio-text);
}

.studio-sidebar__item:focus-visible {
  outline: 2px solid var(--studio-focus);
  outline-offset: 2px;
}

.studio-sidebar__item--current {
  background: var(--studio-surface);
  color: var(--studio-text);
  box-shadow: var(--studio-shadow-control);
}

.studio-sidebar--collapsed .studio-sidebar__item {
  justify-content: center;
  padding: 0;
}

.studio-sidebar--collapsed .studio-sidebar__item-label {
  display: none;
}

.studio-sidebar__collapse {
  display: inline-flex;
  width: var(--studio-control-height-xs);
  height: var(--studio-control-height-xs);
  align-items: center;
  justify-content: center;
  margin-top: auto;
  border: 0;
  border-radius: var(--studio-radius-control);
  background: transparent;
  color: var(--studio-text-muted);
  transition:
    background-color 160ms ease,
    color 160ms ease;
}

.studio-sidebar__collapse:hover {
  background: var(--studio-surface-raised);
  color: var(--studio-text);
}

.studio-sidebar__collapse:focus-visible {
  outline: 2px solid var(--studio-focus);
  outline-offset: 2px;
}

@media (max-width: 720px) {
  .studio-sidebar {
    display: block;
    overflow: visible;
    padding: 0 14px 12px;
  }

  .studio-sidebar__nav {
    display: flex;
    overflow-x: auto;
    gap: 6px;
  }

  .studio-sidebar__item {
    flex: 0 0 auto;
  }

  .studio-sidebar__collapse {
    display: none;
  }
}
</style>
