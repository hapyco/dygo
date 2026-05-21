<script setup lang="ts">
import { Check, ChevronDown } from '@lucide/vue'
import {
  SelectContent,
  SelectIcon,
  SelectItem,
  SelectItemIndicator,
  SelectItemText,
  SelectPortal,
  SelectRoot,
  SelectTrigger,
  SelectValue,
  SelectViewport,
} from 'reka-ui'

import type { ControlSize, FieldOption } from '../types'

const props = withDefaults(defineProps<{
  id?: string
  modelValue?: string
  name?: string
  options: FieldOption[]
  placeholder?: string
  size?: ControlSize
  describedBy?: string
  disabled?: boolean
  readonly?: boolean
  invalid?: boolean
}>(), {
  size: 'md',
  disabled: false,
  readonly: false,
  invalid: false,
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

function updateValue(value: unknown) {
  if (typeof value === 'string') {
    emit('update:modelValue', value)
  }
}
</script>

<template>
  <SelectRoot
    :model-value="modelValue"
    :name="name"
    :disabled="disabled || readonly"
    @update:model-value="updateValue"
  >
    <SelectTrigger
      :id="id"
      class="d-select"
      :class="{ 'd-select--invalid': invalid }"
      :data-size="size"
      :data-readonly="readonly ? '' : undefined"
      :aria-invalid="invalid ? 'true' : undefined"
      :aria-describedby="describedBy || undefined"
      :aria-readonly="readonly ? 'true' : undefined"
    >
      <SelectValue :placeholder="placeholder ?? 'Select'" />
      <SelectIcon class="d-select__icon" aria-hidden="true">
        <ChevronDown :size="15" :stroke-width="1.8" />
      </SelectIcon>
    </SelectTrigger>

    <SelectPortal>
      <SelectContent class="d-select-content" position="popper" :side-offset="6">
        <SelectViewport class="d-select-content__viewport">
          <SelectItem
            v-for="option in props.options"
            :key="option.value"
            class="d-select-item"
            :value="option.value"
            :disabled="option.disabled"
          >
            <SelectItemText>{{ option.label }}</SelectItemText>
            <SelectItemIndicator class="d-select-item__indicator">
              <Check :size="13" :stroke-width="2" aria-hidden="true" />
            </SelectItemIndicator>
          </SelectItem>
        </SelectViewport>
      </SelectContent>
    </SelectPortal>
  </SelectRoot>
</template>

<style scoped>
.d-select {
  display: inline-flex;
  width: 100%;
  min-height: var(--studio-control-height-md);
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  border: 1px solid var(--studio-border);
  border-radius: var(--studio-radius-control);
  background: var(--studio-control-bg);
  box-shadow: var(--studio-shadow-control);
  color: var(--studio-text);
  padding: 0 10px 0 11px;
  font-size: 14px;
  line-height: 1;
  transition:
    background-color 160ms ease,
    border-color 160ms ease,
    box-shadow 160ms ease;
}

.d-select[data-size='sm'] {
  min-height: var(--studio-control-height-sm);
  padding-inline: 9px;
  font-size: 13px;
}

.d-select:hover:not([data-disabled]) {
  border-color: var(--studio-border-strong);
  background: var(--studio-control-bg-hover);
}

.d-select:focus-visible {
  outline: none;
  border-color: var(--studio-focus);
  box-shadow: var(--studio-focus-ring);
}

.d-select[data-disabled] {
  background: var(--studio-control-bg-disabled);
  color: var(--studio-text-subtle);
}

.d-select[data-readonly] {
  background: var(--studio-control-bg-readonly);
  color: var(--studio-text-muted);
  opacity: 1;
}

.d-select--invalid {
  border-color: var(--studio-danger);
}

.d-select--invalid:focus-visible {
  border-color: var(--studio-danger);
  box-shadow: var(--studio-danger-ring);
}

.d-select__icon {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  color: var(--studio-text-subtle);
}

.d-select-content {
  z-index: 40;
  min-width: var(--reka-select-trigger-width);
  overflow: hidden;
  border: 1px solid var(--studio-border);
  border-radius: var(--studio-radius-control);
  background: var(--studio-surface);
  box-shadow: var(--studio-shadow);
  padding: 4px;
}

.d-select-content__viewport {
  display: grid;
  gap: 2px;
}

.d-select-item {
  position: relative;
  display: flex;
  min-height: 32px;
  align-items: center;
  border-radius: 5px;
  color: var(--studio-text);
  font-size: 13px;
  line-height: 1;
  padding: 0 28px 0 9px;
  outline: none;
}

.d-select-item[data-highlighted] {
  background: var(--studio-surface-raised);
}

.d-select-item[data-state='checked'] {
  background: var(--studio-accent-soft);
  color: var(--studio-accent-strong);
}

.d-select-item[data-disabled] {
  color: var(--studio-text-subtle);
}

.d-select-item__indicator {
  position: absolute;
  right: 9px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--studio-accent-strong);
}
</style>
