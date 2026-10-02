<script setup lang="ts">
import { Check } from '@lucide/vue'
import {
  ContextMenuCheckboxItem,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuItemIndicator,
  ContextMenuLabel,
  ContextMenuPortal,
  ContextMenuRoot,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from 'reka-ui'

import type { DropdownMenuItemModel } from '../types'

withDefaults(defineProps<{
  items?: DropdownMenuItemModel[]
  panelClass?: string
}>(), {
  panelClass: '',
})

const emit = defineEmits<{
  select: [key: string]
  'update:checked': [key: string, checked: boolean]
  'update:open': [value: boolean]
}>()

function preventCheckboxClose(event: Event) {
  event.preventDefault()
}
</script>

<template>
  <ContextMenuRoot @update:open="emit('update:open', $event)">
    <ContextMenuTrigger as-child>
      <slot name="trigger" />
    </ContextMenuTrigger>

    <ContextMenuPortal>
      <ContextMenuContent
        :class="panelClass || 'd-dropdown-menu__content'"
        @close-auto-focus.prevent
      >
        <slot v-if="$slots.default" />

        <template v-else>
          <template v-for="item in items ?? []" :key="item.key">
            <ContextMenuLabel v-if="item.type === 'label'" class="d-dropdown-menu__label">
              {{ item.label }}
            </ContextMenuLabel>

            <ContextMenuSeparator v-else-if="item.type === 'separator'" class="d-dropdown-menu__separator" />

            <ContextMenuCheckboxItem
              v-else-if="item.type === 'checkbox'"
              class="d-dropdown-menu__item d-dropdown-menu__item--checkbox"
              :model-value="item.checked"
              :disabled="item.disabled"
              @select="preventCheckboxClose"
              @update:model-value="emit('update:checked', item.key, Boolean($event))"
            >
              <ContextMenuItemIndicator class="d-dropdown-menu__indicator">
                <Check :size="13" :stroke-width="2.2" aria-hidden="true" />
              </ContextMenuItemIndicator>
              <span>{{ item.label }}</span>
            </ContextMenuCheckboxItem>

            <ContextMenuItem
              v-else
              class="d-dropdown-menu__item"
              :disabled="item.disabled"
              @select="emit('select', item.key)"
            >
              {{ item.label }}
            </ContextMenuItem>
          </template>
        </template>
      </ContextMenuContent>
    </ContextMenuPortal>
  </ContextMenuRoot>
</template>
