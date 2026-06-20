<script setup lang="ts">
import { computed } from 'vue'
import {
  DialogContent,
  DialogDescription,
  DialogOverlay,
  DialogPortal,
  DialogRoot,
  DialogTitle,
} from 'reka-ui'

import { Button } from '@/design'
import { useDialogStore, type StudioDialog, type StudioDialogActionVariant } from './dialogs.store'

const dialogStore = useDialogStore()
const topDialog = computed(() => dialogStore.topDialog)

function onOpenChange(open: boolean) {
  if (!open) {
    dialogStore.dismissTop()
  }
}

function preventOutside(event: Event) {
  event.preventDefault()
}

function preventEscapeWhenRequired(event: Event) {
  if (!topDialog.value?.dismissible) {
    event.preventDefault()
  }
}

function choose(dialog: StudioDialog, key: string) {
  dialogStore.selectAction(dialog.id, key)
}

function buttonVariant(variant: StudioDialogActionVariant): 'primary' | 'secondary' | 'danger' {
  return variant === 'danger' ? 'danger' : variant
}
</script>

<template>
  <DialogRoot :open="Boolean(topDialog)" modal @update:open="onOpenChange">
    <DialogPortal v-if="topDialog">
      <DialogOverlay class="studio-dialog__overlay" />
      <DialogContent
        class="studio-dialog"
        :data-type="topDialog.type"
        @escape-key-down="preventEscapeWhenRequired"
        @pointer-down-outside="preventOutside"
        @interact-outside="preventOutside"
      >
        <DialogTitle class="studio-dialog__title">
          {{ topDialog.title }}
        </DialogTitle>
        <DialogDescription v-if="topDialog.content" class="studio-dialog__content">
          {{ topDialog.content }}
        </DialogDescription>
        <div class="studio-dialog__actions">
          <Button
            v-for="action in topDialog.actions"
            :key="action.key"
            :variant="buttonVariant(action.variant)"
            size="sm"
            @click="choose(topDialog, action.key)"
          >
            {{ action.label }}
          </Button>
        </div>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>

<style scoped>
.studio-dialog__overlay {
  position: fixed;
  inset: 0;
  z-index: 80;
  background: oklch(18% 0.02 246 / 0.34);
}

.studio-dialog {
  position: fixed;
  z-index: 81;
  top: 50%;
  left: 50%;
  width: min(calc(100vw - 32px), 420px);
  transform: translate(-50%, -50%);
  border: 1px solid var(--studio-border);
  border-radius: var(--studio-radius-sheet);
  background: var(--studio-surface);
  box-shadow: var(--studio-shadow);
  padding: 18px;
  display: grid;
  gap: 12px;
}

.studio-dialog:focus-visible {
  outline: 2px solid var(--studio-focus);
  outline-offset: 2px;
}

.studio-dialog__title {
  color: var(--studio-text);
  font-size: 16px;
  font-weight: 700;
  line-height: 1.25;
  margin: 0;
}

.studio-dialog__content {
  color: var(--studio-text-muted);
  font-size: 13px;
  line-height: 1.5;
  margin: 0;
  white-space: pre-wrap;
}

.studio-dialog__actions {
  display: flex;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 8px;
  padding-top: 4px;
}
</style>
