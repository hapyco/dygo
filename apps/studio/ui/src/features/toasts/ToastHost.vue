<script setup lang="ts">
import { computed } from 'vue'
import { X } from '@lucide/vue'

import { useToastStore, type StudioToastType } from './toasts.store'

const toastStore = useToastStore()
const visibleToasts = computed(() => toastStore.toasts)

function dismiss(id: number) {
  toastStore.dismiss(id)
}

function labelForType(type: StudioToastType): string {
  return type === 'danger' ? 'Error' : type
}
</script>

<template>
  <div v-if="visibleToasts.length" class="studio-toast-host" aria-live="polite" aria-atomic="false">
    <article
      v-for="toast in visibleToasts"
      :key="toast.id"
      class="studio-toast"
      :data-type="toast.type"
    >
      <div class="studio-toast__body">
        <div class="studio-toast__kicker">
          {{ labelForType(toast.type) }}
        </div>
        <h2 class="studio-toast__title">
          {{ toast.title }}
        </h2>
        <p v-if="toast.content" class="studio-toast__content">
          {{ toast.content }}
        </p>
      </div>
      <button class="studio-toast__close" type="button" aria-label="Dismiss toast" @click="dismiss(toast.id)">
        <X :size="14" aria-hidden="true" />
      </button>
    </article>
  </div>
</template>

<style scoped>
.studio-toast-host {
  position: fixed;
  right: 16px;
  bottom: 16px;
  z-index: 70;
  display: grid;
  gap: 8px;
  width: min(calc(100vw - 32px), 340px);
}

.studio-toast {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
  border: 1px solid var(--studio-border);
  border-left: 3px solid var(--studio-text-subtle);
  border-radius: var(--studio-radius-sheet);
  background: var(--studio-surface);
  box-shadow: var(--studio-shadow);
  padding: 12px;
}

.studio-toast[data-type='success'] {
  border-left-color: var(--studio-success);
}

.studio-toast[data-type='warning'] {
  border-left-color: var(--studio-warning);
}

.studio-toast[data-type='danger'] {
  border-left-color: var(--studio-danger);
}

.studio-toast__body {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.studio-toast__kicker {
  color: var(--studio-text-subtle);
  font-size: 10px;
  font-weight: 700;
  line-height: 1;
  text-transform: uppercase;
}

.studio-toast__title {
  color: var(--studio-text);
  font-size: 13px;
  font-weight: 700;
  line-height: 1.35;
  margin: 0;
}

.studio-toast__content {
  color: var(--studio-text-muted);
  font-size: 12px;
  line-height: 1.45;
  margin: 0;
  white-space: pre-wrap;
}

.studio-toast__close {
  display: inline-grid;
  width: 24px;
  height: 24px;
  place-items: center;
  border: 0;
  border-radius: var(--studio-radius-control);
  background: transparent;
  color: var(--studio-text-subtle);
}

.studio-toast__close:hover {
  background: var(--studio-surface-raised);
  color: var(--studio-text);
}

.studio-toast__close:focus-visible {
  outline: 2px solid var(--studio-focus);
  outline-offset: 2px;
}
</style>
