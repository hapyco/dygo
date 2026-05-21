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
  actions?: PageHeaderAction[]
}>(), {
  showHeader: false,
  showTitle: true,
  showActions: true,
  actions: () => [],
})

const slots = useSlots()

const hasHeaderTitle = computed(() => props.showTitle && Boolean(props.title || slots.title))
const hasHeaderActions = computed(() => props.showActions && (props.actions.length > 0 || Boolean(slots.actions)))
</script>

<template>
  <main class="studio-page-sheet" :aria-labelledby="props.labelledBy" :aria-label="props.ariaLabel">
    <PageHeader
      v-if="props.showHeader"
      class="studio-page-sheet__header"
      :title="props.title"
      :title-id="props.titleId"
      :actions="props.actions"
      :show-title="hasHeaderTitle"
      :show-actions="hasHeaderActions"
    >
      <template #title>
        <slot name="title" />
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
  height: 100%;
  min-height: 0;
  overflow-x: hidden;
  overflow-y: auto;
  border: 1px solid var(--studio-border);
  border-radius: var(--studio-radius-sheet) var(--studio-radius-sheet) 0 0;
  background: var(--studio-surface);
  box-shadow: var(--studio-shadow-sheet);
}

.studio-page-sheet__header {
  padding: 28px 38px 0;
}

@media (max-width: 720px) {
  .studio-page-sheet {
    border-radius: var(--studio-radius-sheet) var(--studio-radius-sheet) 0 0;
  }

  .studio-page-sheet__header {
    padding: 24px 22px 0;
  }
}
</style>
