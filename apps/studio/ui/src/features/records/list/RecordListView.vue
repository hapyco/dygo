<script setup lang="ts">
import { computed } from 'vue'

import type { MetadataField } from '@/features/metadata/metadata.api'

const props = defineProps<{
  entityLabel: string
  fields?: MetadataField[]
}>()

const visibleFields = computed(() => props.fields?.slice(0, 8) ?? [])
const remainingFieldCount = computed(() => Math.max((props.fields?.length ?? 0) - visibleFields.value.length, 0))
</script>

<template>
  <section class="record-list-view" aria-label="Record list view">
    <div class="record-list-view__placeholder">
      <p class="record-list-view__eyebrow">List view</p>
      <p class="record-list-view__copy">
        Metadata columns, table controls, saved views, filters, and row actions will render here for {{ entityLabel }}.
      </p>
      <div v-if="visibleFields.length > 0" class="record-list-view__fields" aria-label="Metadata fields">
        <span v-for="field in visibleFields" :key="field.name" class="record-list-view__field">
          {{ field.label || field.name }}
        </span>
        <span v-if="remainingFieldCount > 0" class="record-list-view__field record-list-view__field--muted">
          +{{ remainingFieldCount }} more
        </span>
      </div>
    </div>
  </section>
</template>

<style scoped>
.record-list-view {
  min-width: 0;
}

.record-list-view__placeholder {
  display: grid;
  min-height: 220px;
  align-content: start;
  gap: 8px;
  border: 1px dashed var(--studio-border);
  border-radius: var(--studio-radius-panel);
  background: var(--studio-surface-raised);
  padding: 18px;
}

.record-list-view__eyebrow {
  margin: 0;
  color: var(--studio-text-subtle);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0;
  line-height: 1.2;
}

.record-list-view__copy {
  max-width: 62ch;
  margin: 0;
  color: var(--studio-text-muted);
  font-size: 14px;
  line-height: 1.5;
}

.record-list-view__fields {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding-top: 8px;
}

.record-list-view__field {
  display: inline-flex;
  min-height: 24px;
  align-items: center;
  border: 1px solid var(--studio-border);
  border-radius: var(--studio-radius-control);
  background: var(--studio-surface);
  color: var(--studio-text-muted);
  font-size: 12px;
  font-weight: 600;
  line-height: 1;
  padding: 0 8px;
}

.record-list-view__field--muted {
  color: var(--studio-text-subtle);
}
</style>
