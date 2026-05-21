<script setup lang="ts">
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
    <span class="d-search-box__icon" aria-hidden="true" />
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
  width: 13px;
  height: 13px;
  border: 1.7px solid var(--studio-text-subtle);
  border-radius: 999px;
  transform: translateY(-50%);
  pointer-events: none;
}

.d-search-box__icon::after {
  position: absolute;
  right: -4px;
  bottom: -3px;
  width: 6px;
  height: 1.7px;
  border-radius: 999px;
  background: var(--studio-text-subtle);
  content: '';
  transform: rotate(45deg);
  transform-origin: center;
}

.d-search-box__input {
  padding-left: 32px;
}
</style>
