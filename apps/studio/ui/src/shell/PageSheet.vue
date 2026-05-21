<script setup lang="ts">
import { computed, useSlots } from 'vue'

import PageHeader from './PageHeader.vue'
import type { PageHeaderAction } from './types'

const props = withDefaults(defineProps<{
  labelledBy?: string
  ariaLabel?: string
  showHeader?: boolean
  showTitle?: boolean
  showActions?: boolean
  title?: string
  titleId?: string
  eyebrow?: string
  summary?: string
  actions?: PageHeaderAction[]
}>(), {
  showHeader: false,
  showTitle: true,
  showActions: true,
  actions: () => [],
})

const slots = useSlots()

const hasHeaderEyebrow = computed(() => Boolean(props.eyebrow || slots.eyebrow))
const hasHeaderTitle = computed(() => props.showTitle && Boolean(props.title || slots.title))
const hasHeaderSummary = computed(() => Boolean(props.summary || slots.summary))
const hasHeaderActions = computed(() => props.showActions && (props.actions.length > 0 || Boolean(slots.actions)))
</script>

<template>
  <main class="studio-page-sheet" :aria-labelledby="props.labelledBy" :aria-label="props.ariaLabel">
    <PageHeader
      v-if="props.showHeader"
      class="studio-page-sheet__header"
      :title="props.title"
      :title-id="props.titleId"
      :eyebrow="props.eyebrow"
      :summary="props.summary"
      :actions="props.actions"
      :show-eyebrow="hasHeaderEyebrow"
      :show-title="hasHeaderTitle"
      :show-summary="hasHeaderSummary"
      :show-actions="hasHeaderActions"
    >
      <template #eyebrow>
        <slot name="eyebrow" />
      </template>
      <template #title>
        <slot name="title" />
      </template>
      <template #summary>
        <slot name="summary" />
      </template>
      <template #actions>
        <slot name="actions" />
      </template>
    </PageHeader>

    <slot />
  </main>
</template>

<style scoped>
.studio-page-sheet {
  min-height: calc(100vh - var(--studio-shell-header-height) - var(--studio-shell-bottom-gutter));
  overflow: hidden;
  border: 1px solid var(--studio-border);
  border-radius: var(--studio-radius-sheet);
  background: var(--studio-surface);
  box-shadow: var(--studio-shadow-sheet);
}

.studio-page-sheet__header {
  padding: 28px 38px 0;
}

@media (max-width: 720px) {
  .studio-page-sheet {
    min-height: calc(100vh - var(--studio-shell-header-height) - 82px);
    border-radius: var(--studio-radius-sheet);
  }

  .studio-page-sheet__header {
    padding: 24px 22px 0;
  }
}
</style>
