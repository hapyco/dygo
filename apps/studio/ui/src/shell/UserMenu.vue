<script setup lang="ts">
import { LogOut } from '@lucide/vue'
import {
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuPortal,
  DropdownMenuRoot,
  DropdownMenuTrigger,
} from 'reka-ui'
import { useRouter } from 'vue-router'

import Avatar from '@/design/atoms/Avatar.vue'
import { RouteName } from '@/router/routes'
import { useAuthStore } from '@/stores/auth.store'

withDefaults(defineProps<{
  userName?: string
  userAvatarUrl?: string
}>(), {
  userName: 'Studio user',
})

const router = useRouter()
const authStore = useAuthStore()

async function logout() {
  await authStore.logout()
  await router.replace({ name: RouteName.Login })
}
</script>

<template>
  <DropdownMenuRoot>
    <DropdownMenuTrigger as-child>
      <button class="studio-user-menu__trigger" type="button" :aria-label="`${userName} menu`">
        <Avatar :name="userName" :image-url="userAvatarUrl" />
      </button>
    </DropdownMenuTrigger>

    <DropdownMenuPortal>
      <DropdownMenuContent
        class="studio-user-menu__content"
        align="end"
        :side-offset="8"
      >
        <DropdownMenuItem class="studio-user-menu__item" @select="logout">
          <LogOut :size="14" :stroke-width="1.8" aria-hidden="true" />
          <span>Logout</span>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenuPortal>
  </DropdownMenuRoot>
</template>

<style scoped>
.studio-user-menu__trigger {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: 999px;
  background: transparent;
  color: inherit;
  padding: 0;
}

.studio-user-menu__trigger:focus-visible {
  outline: 2px solid var(--studio-focus);
  outline-offset: 2px;
}

.studio-user-menu__content {
  z-index: 50;
  min-width: 160px;
  overflow: hidden;
  border: 1px solid var(--studio-border);
  border-radius: var(--studio-radius-control);
  background: var(--studio-surface);
  box-shadow: var(--studio-shadow-sheet);
  padding: 5px;
}

.studio-user-menu__item {
  display: flex;
  min-height: 30px;
  align-items: center;
  gap: 8px;
  border-radius: 5px;
  color: var(--studio-text-muted);
  font-size: 13px;
  font-weight: 500;
  line-height: 1;
  outline: none;
  padding: 0 8px;
  user-select: none;
}

.studio-user-menu__item[data-highlighted] {
  background: var(--studio-surface-raised);
  color: var(--studio-text);
}
</style>
