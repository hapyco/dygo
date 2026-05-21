<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import {
  ChartNoAxesColumn,
  Command,
  Database,
  Home,
  Search,
  Settings,
  Workflow,
} from '@lucide/vue'

const open = ref(false)
const searchInput = ref<HTMLInputElement | null>(null)

const commands = [
  { label: 'Home', to: '/', icon: Home },
  { label: 'Records', to: '/records', icon: Database },
  { label: 'Reports', to: '/reports', icon: ChartNoAxesColumn },
  { label: 'Workflows', to: '/workflows', icon: Workflow },
  { label: 'Settings', to: '/settings', icon: Settings },
]

async function openMenu() {
  open.value = true
  await nextTick()
  searchInput.value?.focus()
}

function closeMenu() {
  open.value = false
}

function handleKeydown(event: KeyboardEvent) {
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault()
    void openMenu()
    return
  }

  if (event.key === 'Escape' && open.value) {
    event.preventDefault()
    closeMenu()
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <div class="studio-command-menu">
    <button
      class="studio-command-menu__trigger"
      type="button"
      aria-haspopup="dialog"
      :aria-expanded="open"
      @click="openMenu"
    >
      <Search class="studio-command-menu__search-icon" :size="14" :stroke-width="1.8" aria-hidden="true" />
      <span class="studio-command-menu__placeholder">Search</span>
      <span class="studio-command-menu__shortcut" aria-hidden="true">
        <Command :size="12" :stroke-width="1.9" />
        <kbd>K</kbd>
      </span>
      <span class="studio-command-menu__shortcut-label">Command K</span>
    </button>

    <Teleport to="body">
      <div
        v-if="open"
        class="studio-command-menu__overlay"
        role="presentation"
        @click.self="closeMenu"
      >
        <section
          class="studio-command-menu__dialog"
          role="dialog"
          aria-modal="true"
          aria-label="Command menu"
        >
          <div class="studio-command-menu__input-wrap">
            <Search class="studio-command-menu__dialog-icon" :size="16" :stroke-width="1.8" aria-hidden="true" />
            <input
              ref="searchInput"
              class="studio-command-menu__input"
              type="search"
              placeholder="Search commands"
              @keydown.esc.prevent="closeMenu"
            >
            <span class="studio-command-menu__dialog-shortcut" aria-hidden="true">
              <Command :size="12" :stroke-width="1.9" />
              <kbd>K</kbd>
            </span>
          </div>

          <div class="studio-command-menu__list" role="listbox" aria-label="Commands">
            <RouterLink
              v-for="command in commands"
              :key="command.label"
              class="studio-command-menu__item"
              :to="command.to"
              role="option"
              @click="closeMenu"
            >
              <component
                :is="command.icon"
                class="studio-command-menu__item-icon"
                :size="16"
                :stroke-width="1.8"
                aria-hidden="true"
              />
              <span>{{ command.label }}</span>
            </RouterLink>
          </div>
        </section>
      </div>
    </Teleport>
  </div>
</template>

<style>
.studio-command-menu {
  min-width: 0;
}

.studio-command-menu__trigger {
  display: flex;
  width: 100%;
  height: var(--studio-control-height-sm);
  min-width: 0;
  align-items: center;
  gap: 8px;
  border: 1px solid var(--studio-border);
  border-radius: var(--studio-radius-control);
  background: var(--studio-control-bg);
  color: var(--studio-text-subtle);
  padding: 0 6px 0 10px;
  text-align: left;
  box-shadow: var(--studio-shadow-control);
  transition:
    background-color 160ms ease,
    border-color 160ms ease,
    box-shadow 160ms ease;
}

.studio-command-menu__trigger:hover {
  border-color: var(--studio-border-strong);
  background: var(--studio-control-bg-hover);
}

.studio-command-menu__trigger:focus-visible {
  border-color: var(--studio-focus);
  outline: none;
  box-shadow: var(--studio-focus-ring);
}

.studio-command-menu__search-icon {
  flex: 0 0 auto;
}

.studio-command-menu__placeholder {
  min-width: 0;
  flex: 1 1 auto;
  overflow: hidden;
  font-size: 12px;
  font-weight: 500;
  line-height: 1;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.studio-command-menu__shortcut,
.studio-command-menu__dialog-shortcut {
  display: inline-flex;
  height: 20px;
  flex: 0 0 auto;
  align-items: center;
  gap: 3px;
  border: 1px solid var(--studio-border);
  border-radius: 5px;
  background: var(--studio-neutral-soft);
  color: var(--studio-text-muted);
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
  padding: 0 5px;
}

.studio-command-menu__shortcut kbd,
.studio-command-menu__dialog-shortcut kbd {
  font: inherit;
}

.studio-command-menu__shortcut-label {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
  white-space: nowrap;
}

.studio-command-menu__overlay {
  position: fixed;
  z-index: 60;
  inset: 0;
  display: grid;
  place-items: start center;
  background: oklch(0.225 0.018 246 / 0.18);
  padding: 86px 18px 18px;
}

.studio-command-menu__dialog {
  width: min(560px, 100%);
  overflow: hidden;
  border: 1px solid var(--studio-border);
  border-radius: var(--studio-radius-panel);
  background: var(--studio-surface);
  box-shadow: var(--studio-shadow-sheet);
}

.studio-command-menu__input-wrap {
  display: flex;
  height: 46px;
  align-items: center;
  gap: 10px;
  border-bottom: 1px solid var(--studio-border);
  padding: 0 10px 0 14px;
}

.studio-command-menu__dialog-icon {
  flex: 0 0 auto;
  color: var(--studio-text-subtle);
}

.studio-command-menu__input {
  min-width: 0;
  flex: 1 1 auto;
  border: 0;
  background: transparent;
  color: var(--studio-text);
  font-size: 14px;
  outline: none;
}

.studio-command-menu__input::placeholder {
  color: var(--studio-text-subtle);
}

.studio-command-menu__list {
  display: grid;
  gap: 3px;
  padding: 7px;
}

.studio-command-menu__item {
  display: flex;
  min-height: 36px;
  align-items: center;
  gap: 10px;
  border-radius: var(--studio-radius-control);
  color: var(--studio-text-muted);
  font-size: 13px;
  font-weight: 600;
  padding: 0 9px;
  text-decoration: none;
}

.studio-command-menu__item:hover,
.studio-command-menu__item:focus-visible {
  background: var(--studio-surface-raised);
  color: var(--studio-text);
  outline: none;
}

.studio-command-menu__item-icon {
  flex: 0 0 auto;
}
</style>
