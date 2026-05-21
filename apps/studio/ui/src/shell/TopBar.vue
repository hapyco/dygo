<script setup lang="ts">
import { Bell } from '@lucide/vue'

import LogoMark from '@/design/atoms/LogoMark.vue'
import Breadcrumbs from './Breadcrumbs.vue'
import CommandMenu from './CommandMenu.vue'
import UserMenu from './UserMenu.vue'

withDefaults(defineProps<{
  brandLabel?: string
  brandMark?: string
  companyName?: string
  userName?: string
  userAvatarUrl?: string
}>(), {
  brandLabel: 'dygo Studio',
  brandMark: 'd',
  companyName: 'Umami Smokehouse',
  userName: 'Studio user',
})
</script>

<template>
  <header class="studio-top-bar">
    <div class="studio-top-bar__brand">
      <LogoMark :label="brandLabel" :mark="brandMark" />
      <span class="studio-top-bar__company">{{ companyName }}</span>
    </div>

    <div class="studio-top-bar__bar">
      <Breadcrumbs class="studio-top-bar__breadcrumbs" />
      <CommandMenu class="studio-top-bar__search" />

      <div class="studio-top-bar__right">
        <slot name="actions" />
        <button class="studio-top-bar__notification" type="button" aria-label="Notifications">
          <Bell :size="16" :stroke-width="1.8" aria-hidden="true" />
        </button>
        <UserMenu :user-name="userName" :user-avatar-url="userAvatarUrl" />
      </div>
    </div>
  </header>
</template>

<style scoped>
.studio-top-bar {
  display: grid;
  min-height: var(--studio-shell-header-height);
  grid-template-columns: var(--studio-shell-sidebar-width) minmax(0, 1fr);
  align-items: center;
}

.studio-top-bar__brand {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 10px;
  padding-left: var(--studio-shell-gutter);
  padding-right: 14px;
}

.studio-top-bar__company {
  min-width: 0;
  overflow: hidden;
  color: var(--studio-text);
  font-size: 13px;
  font-weight: 700;
  line-height: 1.15;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.studio-top-bar__bar {
  display: grid;
  min-width: 0;
  grid-template-columns: auto minmax(0, 1fr) minmax(220px, 312px) auto;
  align-items: center;
  gap: 14px;
  padding-right: var(--studio-shell-gutter);
}

.studio-top-bar__search {
  grid-column: 3;
}

.studio-top-bar__right {
  display: inline-flex;
  grid-column: 4;
  align-items: center;
  justify-self: end;
  gap: 10px;
}

.studio-top-bar__notification {
  position: relative;
  display: inline-flex;
  width: 32px;
  height: 32px;
  align-items: center;
  justify-content: center;
  border: 1px solid transparent;
  border-radius: var(--studio-radius-control);
  background: transparent;
  color: var(--studio-text-muted);
  transition:
    background-color 160ms ease,
    border-color 160ms ease,
    color 160ms ease;
}

.studio-top-bar__notification:hover {
  background: var(--studio-surface-raised);
  color: var(--studio-text);
}

.studio-top-bar__notification:focus-visible {
  outline: 2px solid var(--studio-focus);
  outline-offset: 2px;
}

@media (max-width: 720px) {
  .studio-top-bar {
    grid-template-columns: minmax(0, 1fr) auto;
    padding-inline: 14px;
  }

  .studio-top-bar__brand {
    padding: 0;
  }

  .studio-top-bar__bar {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    padding: 0;
  }

  .studio-top-bar__breadcrumbs,
  .studio-top-bar__search {
    display: none;
  }
}
</style>
