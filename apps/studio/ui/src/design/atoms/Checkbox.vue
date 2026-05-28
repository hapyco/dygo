<script setup lang="ts">
import { Check } from '@lucide/vue'
import { CheckboxIndicator, CheckboxRoot } from 'reka-ui'

withDefaults(
  defineProps<{
    id?: string
    modelValue?: boolean
    name?: string
    describedBy?: string
    required?: boolean
    disabled?: boolean
    readonly?: boolean
    invalid?: boolean
  }>(),
  {
    modelValue: false,
    disabled: false,
    readonly: false,
    invalid: false,
  },
)

defineEmits<{
  'update:modelValue': [value: boolean]
}>()
</script>

<template>
  <CheckboxRoot
    :id="id"
    class="d-checkbox"
    :model-value="modelValue"
    :name="name"
    :disabled="disabled || readonly"
    :required="required"
    :aria-invalid="invalid ? 'true' : undefined"
    :aria-describedby="describedBy || undefined"
    :aria-readonly="readonly ? 'true' : undefined"
    :data-readonly="readonly ? '' : undefined"
    :class="{ 'd-checkbox--invalid': invalid }"
    @update:model-value="$emit('update:modelValue', Boolean($event))"
  >
    <CheckboxIndicator class="d-checkbox__indicator">
      <Check :size="13" :stroke-width="2.4" aria-hidden="true" />
    </CheckboxIndicator>
  </CheckboxRoot>
</template>

<style scoped>
.d-checkbox {
  display: inline-flex;
  width: 16px;
  height: 16px;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  box-sizing: border-box;
  border: 1px solid var(--studio-border-strong);
  border-radius: 5px;
  background: var(--studio-control-bg);
  color: oklch(0.99 0.004 246);
  box-shadow: var(--studio-shadow-control);
  line-height: 0;
  vertical-align: middle;
  transition:
    background-color 160ms ease,
    border-color 160ms ease,
    box-shadow 160ms ease;
}

.d-checkbox:hover:not([data-disabled]) {
  border-color: var(--studio-accent);
}

.d-checkbox:focus-visible {
  outline: 2px solid var(--studio-focus);
  outline-offset: 2px;
}

.d-checkbox[data-state='checked'] {
  border-color: var(--studio-accent);
  background: var(--studio-accent);
}

.d-checkbox[data-disabled] {
  opacity: 0.58;
}

.d-checkbox[data-readonly] {
  background: var(--studio-control-bg-readonly);
  opacity: 1;
}

.d-checkbox--invalid {
  border-color: var(--studio-danger);
}

.d-checkbox__indicator {
  display: inline-flex;
  width: 14px;
  height: 14px;
  align-items: center;
  justify-content: center;
}
</style>
