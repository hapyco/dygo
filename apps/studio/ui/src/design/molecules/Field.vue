<script setup lang="ts">
import { computed } from 'vue'

import Label from '../atoms/Label.vue'

const props = withDefaults(defineProps<{
  id: string
  label: string
  hint?: string
  error?: string
  required?: boolean
  disabled?: boolean
  readonly?: boolean
}>(), {
  required: false,
  disabled: false,
  readonly: false,
})

const hintId = computed(() => (props.hint ? `${props.id}-hint` : undefined))
const errorId = computed(() => (props.error ? `${props.id}-error` : undefined))
const describedBy = computed(() => [hintId.value, errorId.value].filter(Boolean).join(' ') || undefined)
</script>

<template>
  <div
    class="d-field"
    :class="{
      'd-field--disabled': disabled,
      'd-field--readonly': readonly,
      'd-field--invalid': Boolean(error),
    }"
  >
    <div class="d-field__header">
      <Label :for="id">
        {{ label }}
        <span v-if="required" class="d-field__required" aria-hidden="true">*</span>
      </Label>
      <span v-if="hint" :id="hintId" class="d-field__hint">{{ hint }}</span>
    </div>
    <slot :id="id" :invalid="Boolean(error)" :described-by="describedBy" />
    <p v-if="error" :id="errorId" class="d-field__error" role="alert">{{ error }}</p>
  </div>
</template>

<style scoped>
.d-field {
  display: grid;
  gap: 7px;
}

.d-field__header {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
}

.d-field__required {
  color: var(--studio-danger);
}

.d-field__hint {
  color: var(--studio-text-subtle);
  font-size: 12px;
  line-height: 1.2;
}

.d-field__error {
  margin: 0;
  color: var(--studio-danger);
  font-size: 12px;
  line-height: 1.35;
}

.d-field--disabled {
  color: var(--studio-text-subtle);
}

.d-field--readonly .d-field__hint {
  color: var(--studio-text-muted);
}
</style>
