<script setup lang="ts">
import { Check, ChevronDown } from '@lucide/vue'
import {
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuItemIndicator,
  DropdownMenuLabel,
  DropdownMenuPortal,
  DropdownMenuRoot,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from 'reka-ui'

import Button from '../atoms/Button.vue'
import IconButton from '../atoms/IconButton.vue'
import type { DropdownMenuItem as DropdownMenuItemModel } from '../types'

withDefaults(defineProps<{
  label: string
  items: DropdownMenuItemModel[]
  align?: 'start' | 'center' | 'end'
  triggerType?: 'button' | 'icon'
}>(), {
  align: 'end',
  triggerType: 'button',
})

const emit = defineEmits<{
  select: [key: string]
  'update:checked': [key: string, checked: boolean]
}>()

function preventCheckboxClose(event: Event) {
  event.preventDefault()
}
</script>

<template>
  <DropdownMenuRoot>
    <DropdownMenuTrigger as-child>
      <IconButton
        v-if="triggerType === 'icon'"
        class="d-dropdown-menu__trigger"
        type="button"
        variant="secondary"
        :label="label"
      >
        <slot name="trigger">
          <ChevronDown :size="13" :stroke-width="1.9" aria-hidden="true" />
        </slot>
      </IconButton>

      <Button
        v-else
        class="d-dropdown-menu__trigger"
        type="button"
        variant="secondary"
        :aria-label="label"
      >
        <slot name="trigger">
          {{ label }}
          <ChevronDown :size="13" :stroke-width="1.9" aria-hidden="true" />
        </slot>
      </Button>
    </DropdownMenuTrigger>

    <DropdownMenuPortal>
      <DropdownMenuContent
        class="d-dropdown-menu__content"
        :align="align"
        :side-offset="6"
      >
        <template v-for="item in items" :key="item.key">
          <DropdownMenuLabel v-if="item.type === 'label'" class="d-dropdown-menu__label">
            {{ item.label }}
          </DropdownMenuLabel>

          <DropdownMenuSeparator v-else-if="item.type === 'separator'" class="d-dropdown-menu__separator" />

          <DropdownMenuCheckboxItem
            v-else-if="item.type === 'checkbox'"
            class="d-dropdown-menu__item d-dropdown-menu__item--checkbox"
            :model-value="item.checked"
            :disabled="item.disabled"
            @select="preventCheckboxClose"
            @update:model-value="emit('update:checked', item.key, Boolean($event))"
          >
            <DropdownMenuItemIndicator class="d-dropdown-menu__indicator">
              <Check :size="13" :stroke-width="2.2" aria-hidden="true" />
            </DropdownMenuItemIndicator>
            <span>{{ item.label }}</span>
          </DropdownMenuCheckboxItem>

          <DropdownMenuItem
            v-else
            class="d-dropdown-menu__item"
            :disabled="item.disabled"
            @select="emit('select', item.key)"
          >
            {{ item.label }}
          </DropdownMenuItem>
        </template>
      </DropdownMenuContent>
    </DropdownMenuPortal>
  </DropdownMenuRoot>
</template>

<style scoped>
.d-dropdown-menu__trigger {
  flex: 0 0 auto;
}

.d-dropdown-menu__content {
  z-index: 50;
  min-width: 184px;
  max-width: min(280px, calc(100vw - 24px));
  overflow: hidden;
  border: 1px solid var(--studio-border);
  border-radius: var(--studio-radius-control);
  background: var(--studio-surface);
  box-shadow: var(--studio-shadow-sheet);
  padding: 5px;
}

.d-dropdown-menu__label {
  padding: 6px 8px 5px;
  color: var(--studio-text-subtle);
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
  text-transform: uppercase;
}

.d-dropdown-menu__separator {
  height: 1px;
  margin: 5px -5px;
  background: var(--studio-border);
}

.d-dropdown-menu__item {
  display: flex;
  min-height: 28px;
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

.d-dropdown-menu__item--checkbox {
  padding-left: 28px;
  position: relative;
}

.d-dropdown-menu__item[data-highlighted] {
  background: var(--studio-surface-raised);
  color: var(--studio-text);
}

.d-dropdown-menu__item[data-disabled] {
  color: var(--studio-text-subtle);
  opacity: 0.62;
}

.d-dropdown-menu__indicator {
  position: absolute;
  left: 8px;
  display: inline-flex;
  width: 14px;
  align-items: center;
  justify-content: center;
}
</style>
