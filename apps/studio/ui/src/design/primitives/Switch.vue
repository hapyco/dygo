<script setup lang="ts">
import { SwitchRoot, SwitchThumb } from 'reka-ui'

const props = withDefaults(defineProps<{
  id?: string
  modelValue?: boolean
  name?: string
  describedBy?: string
  required?: boolean
  disabled?: boolean
  readonly?: boolean
  invalid?: boolean
}>(), {
  modelValue: false,
  disabled: false,
  readonly: false,
  invalid: false,
})

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

function updateValue(value: boolean) {
  if (!props.readonly) {
    emit('update:modelValue', value)
  }
}
</script>

<template>
  <SwitchRoot
    :id="id"
    class="d-switch"
    :class="{ 'd-switch--invalid': invalid }"
    :model-value="modelValue"
    :name="name"
    :required="required"
    :disabled="disabled || readonly"
    :aria-invalid="invalid ? 'true' : undefined"
    :aria-describedby="describedBy || undefined"
    :aria-readonly="readonly ? 'true' : undefined"
    :data-readonly="readonly ? '' : undefined"
    @update:model-value="updateValue"
  >
    <SwitchThumb class="d-switch__thumb" />
  </SwitchRoot>
</template>

<style scoped>
.d-switch {
  position: relative;
  display: inline-flex;
  width: 34px;
  height: 20px;
  flex: 0 0 auto;
  align-items: center;
  border: 1px solid var(--studio-border-strong);
  border-radius: 999px;
  background: var(--studio-control-bg-disabled);
  box-shadow: var(--studio-shadow-control);
  padding: 0 2px;
  transition:
    background-color 160ms ease,
    border-color 160ms ease,
    box-shadow 160ms ease;
}

.d-switch:hover:not([data-disabled]) {
  border-color: var(--studio-accent);
}

.d-switch:focus-visible {
  outline: 2px solid var(--studio-focus);
  outline-offset: 2px;
}

.d-switch[data-state='checked'] {
  border-color: var(--studio-accent);
  background: var(--studio-accent);
}

.d-switch[data-disabled] {
  opacity: 0.58;
}

.d-switch[data-readonly] {
  background: var(--studio-control-bg-readonly);
  opacity: 1;
}

.d-switch--invalid {
  border-color: var(--studio-danger);
}

.d-switch__thumb {
  display: block;
  width: 14px;
  height: 14px;
  border-radius: 999px;
  background: var(--studio-surface);
  box-shadow: 0 1px 2px oklch(0.225 0.018 246 / 0.2);
  transform: translateX(0);
  transition: transform 160ms ease;
}

.d-switch[data-state='checked'] .d-switch__thumb {
  transform: translateX(14px);
}
</style>
