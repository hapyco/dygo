<script setup lang="ts">
import { computed, useSlots } from 'vue'

import Button from '@/design/atoms/Button.vue'
import type { PageHeaderAction } from './types'

const props = withDefaults(defineProps<{
  title?: string
  titleId?: string
  eyebrow?: string
  summary?: string
  showEyebrow?: boolean
  showSummary?: boolean
  showTitle?: boolean
  showActions?: boolean
  actions?: PageHeaderAction[]
}>(), {
  showEyebrow: true,
  showSummary: true,
  showTitle: true,
  showActions: true,
  actions: () => [],
})

const slots = useSlots()

const hasTitle = computed(() => props.showTitle && Boolean(props.title || slots.title))
const hasActions = computed(() => props.showActions && (props.actions.length > 0 || Boolean(slots.actions)))
const hasEyebrow = computed(() => props.showEyebrow && Boolean(props.eyebrow || slots.eyebrow))
const hasSummary = computed(() => props.showSummary && Boolean(props.summary || slots.summary))

function runAction(action: PageHeaderAction) {
  if (action.disabled || action.loading) {
    return
  }

  action.onSelect?.()
}
</script>

<template>
  <header
    class="studio-page-header"
    :class="{ 'studio-page-header--with-actions': hasActions }"
  >
    <div class="studio-page-header__main">
      <p v-if="hasEyebrow" class="studio-page-header__eyebrow">
        <slot name="eyebrow">{{ props.eyebrow }}</slot>
      </p>

      <h1 v-if="hasTitle" :id="props.titleId" class="studio-page-header__title">
        <slot name="title">{{ props.title }}</slot>
      </h1>

      <p v-if="hasSummary" class="studio-page-header__summary">
        <slot name="summary">{{ props.summary }}</slot>
      </p>
    </div>

    <div v-if="hasActions" class="studio-page-header__actions">
      <slot name="actions">
        <Button
          v-for="action in props.actions"
          :key="action.label"
          type="button"
          :variant="action.variant ?? 'secondary'"
          size="sm"
          :disabled="action.disabled"
          :loading="action.loading"
          @click="runAction(action)"
        >
          <component
            :is="action.icon"
            v-if="action.icon"
            class="studio-page-header__action-icon"
            :size="15"
            :stroke-width="1.8"
            aria-hidden="true"
          />
          {{ action.label }}
        </Button>
      </slot>
    </div>
  </header>
</template>

<style scoped>
.studio-page-header {
  display: grid;
  min-width: 0;
  gap: 16px;
}

.studio-page-header--with-actions {
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: start;
}

.studio-page-header__main {
  display: grid;
  min-width: 0;
  gap: 10px;
}

.studio-page-header__eyebrow {
  margin: 0;
  color: var(--studio-text-subtle);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0;
  line-height: 1.2;
}

.studio-page-header__title {
  margin: 0;
  color: var(--studio-text);
  font-size: 24px;
  font-weight: 700;
  letter-spacing: 0;
  line-height: 1.16;
}

.studio-page-header__summary {
  max-width: 68ch;
  margin: 0;
  color: var(--studio-text-muted);
  font-size: 15px;
  font-weight: 400;
  letter-spacing: 0;
  line-height: 1.55;
}

.studio-page-header__actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}

.studio-page-header__action-icon {
  flex: 0 0 auto;
}

@media (max-width: 720px) {
  .studio-page-header--with-actions {
    grid-template-columns: minmax(0, 1fr);
  }

  .studio-page-header__actions {
    justify-content: flex-start;
  }
}
</style>
