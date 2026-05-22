<script setup lang="ts">
import { computed, useSlots } from 'vue'

import Button from '@/design/atoms/Button.vue'
import type { PageHeaderAction } from './types'

const props = withDefaults(defineProps<{
  title?: string
  titleId?: string
  showTitle?: boolean
  showActions?: boolean
  actions?: PageHeaderAction[]
}>(), {
  showTitle: true,
  showActions: true,
  actions: () => [],
})

const slots = useSlots()

const hasTitle = computed(() => props.showTitle && Boolean(props.title || slots.title))
const hasActions = computed(() => props.showActions && (props.actions.length > 0 || Boolean(slots.actions)))

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
      <h1 v-if="hasTitle" :id="props.titleId" class="studio-page-header__title">
        <slot name="title">{{ props.title }}</slot>
      </h1>
    </div>

    <div v-if="hasActions" class="studio-page-header__actions">
      <slot name="actions">
        <Button
          v-for="action in props.actions"
          :key="action.label"
          type="button"
          :variant="action.variant ?? 'secondary'"
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
  margin: calc(var(--studio-page-padding) * -1) calc(var(--studio-page-padding) * -1) 0;
  border-bottom: 1px solid var(--studio-border);
  padding: 10px var(--studio-page-padding);
}

.studio-page-header--with-actions {
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
}

.studio-page-header__main {
  min-width: 0;
}

.studio-page-header__title {
  margin: 0;
  color: var(--studio-text);
  font-size: 20px;
  font-weight: 500;
  letter-spacing: 0;
  line-height: 1.16;
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
