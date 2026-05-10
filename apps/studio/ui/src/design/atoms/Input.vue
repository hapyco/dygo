<script setup lang="ts">
defineProps<{
  id?: string
  modelValue?: string
  name?: string
  type?: string
  placeholder?: string
  autocomplete?: string
  required?: boolean
  disabled?: boolean
  invalid?: boolean
}>()

defineEmits<{
  'update:modelValue': [value: string]
}>()
</script>

<template>
  <input
    :id="id"
    class="d-input"
    :class="{ 'd-input--invalid': invalid }"
    :value="modelValue"
    :name="name"
    :type="type ?? 'text'"
    :placeholder="placeholder"
    :autocomplete="autocomplete"
    :required="required"
    :disabled="disabled"
    :aria-invalid="invalid ? 'true' : undefined"
    @input="$emit('update:modelValue', ($event.target as HTMLInputElement).value)"
  />
</template>

<style scoped>
.d-input {
  width: 100%;
  min-height: 38px;
  border: 1px solid var(--studio-border);
  border-radius: 7px;
  background: var(--studio-surface);
  color: var(--studio-text);
  padding: 0 11px;
  font-size: 14px;
  line-height: 1;
  transition:
    border-color 160ms ease,
    box-shadow 160ms ease,
    background-color 160ms ease;
}

.d-input::placeholder {
  color: var(--studio-text-subtle);
}

.d-input:hover:not(:disabled) {
  border-color: var(--studio-border-strong);
}

.d-input:focus {
  outline: none;
}

.d-input:focus-visible {
  border-color: var(--studio-focus);
  box-shadow: 0 0 0 3px oklch(0.61 0.16 248 / 0.16);
}

.d-input:disabled {
  background: var(--studio-surface-raised);
  color: var(--studio-text-subtle);
}

.d-input--invalid {
  border-color: var(--studio-danger);
}

.d-input--invalid:focus-visible {
  border-color: var(--studio-danger);
  box-shadow: 0 0 0 3px oklch(0.55 0.15 28 / 0.14);
}
</style>
