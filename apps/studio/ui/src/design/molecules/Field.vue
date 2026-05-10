<script setup lang="ts">
import Label from '../atoms/Label.vue'

defineProps<{
  id: string
  label: string
  hint?: string
  error?: string
}>()
</script>

<template>
  <div class="d-field">
    <div class="d-field__header">
      <Label :for="id">{{ label }}</Label>
      <span v-if="hint" class="d-field__hint">{{ hint }}</span>
    </div>
    <slot :id="id" :invalid="Boolean(error)" />
    <p v-if="error" class="d-field__error" :id="`${id}-error`">{{ error }}</p>
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
</style>
