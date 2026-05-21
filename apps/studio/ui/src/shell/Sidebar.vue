<script setup lang="ts">
import { RouterLink } from 'vue-router'

import type { ShellNavItem } from './types'

withDefaults(defineProps<{
  ariaLabel?: string
  items?: ShellNavItem[]
}>(), {
  ariaLabel: 'Studio navigation',
  items: () => [],
})
</script>

<template>
  <aside class="studio-sidebar" :aria-label="ariaLabel">
    <nav v-if="items.length > 0" class="studio-sidebar__nav">
      <RouterLink
        v-for="item in items"
        :key="item.label"
        class="studio-sidebar__item"
        :class="{ 'studio-sidebar__item--current': item.current }"
        :to="item.to"
        :aria-current="item.current ? 'page' : undefined"
      >
        {{ item.label }}
      </RouterLink>
    </nav>
    <slot />
  </aside>
</template>

<style scoped>
.studio-sidebar {
  min-width: 0;
  padding: 18px 14px 24px var(--studio-shell-gutter);
}

.studio-sidebar__nav {
  display: grid;
  gap: 4px;
}

.studio-sidebar__item {
  display: flex;
  min-height: 34px;
  align-items: center;
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

@media (max-width: 720px) {
  .studio-sidebar {
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
}
</style>
