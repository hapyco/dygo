<script setup lang="ts">
import { Search } from '@lucide/vue'

import Input from '../atoms/Input.vue'
import type { ControlSize } from '../types'

withDefaults(defineProps<{
  id?: string
  modelValue?: string
  name?: string
  placeholder?: string
  size?: ControlSize
  disabled?: boolean
  readonly?: boolean
}>(), {
  placeholder: 'Search',
  size: 'md',
  disabled: false,
  readonly: false,
})

defineEmits<{
  'update:modelValue': [value: string]
}>()
</script>

<template>
  <div class="d-search-box" role="search">
    <Search class="d-search-box__icon" :size="14" :stroke-width="1.8" aria-hidden="true" />
    <Input
      :id="id"
      class="d-search-box__input"
      :model-value="modelValue"
      :name="name"
      type="search"
      :size="size"
      :placeholder="placeholder"
      :disabled="disabled"
      :readonly="readonly"
      @update:model-value="$emit('update:modelValue', $event)"
    />
  </div>
</template>

<style scoped>
.d-search-box {
  position: relative;
  width: 100%;
}

.d-search-box__icon {
  position: absolute;
  z-index: 1;
  top: 50%;
  left: 11px;
  color: var(--studio-text-subtle);
  transform: translateY(-50%);
  pointer-events: none;
}

.d-search-box__input {
  padding-left: 32px;
}
</style>
