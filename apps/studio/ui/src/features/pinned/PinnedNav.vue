<script setup lang="ts">
import { computed, ref } from 'vue'
import { ChevronDown, ChevronUp, Ellipsis, Pin } from '@lucide/vue'
import { PopoverContent, PopoverPortal, PopoverRoot, PopoverTrigger } from 'reka-ui'
import { RouterLink, useRoute } from 'vue-router'

import { useMetadataEntitiesQuery } from '@/features/metadata/metadata.query'
import { studioPinIsCurrent } from '@/router/current'
import type { PinnedItem } from './pinned'
import { useBootStore } from '@/stores/boot.store'
import { useNavigationStore } from '@/stores/navigation.store'

const props = defineProps<{ collapsed?: boolean }>()
const navigation = useNavigationStore()
const boot = useBootStore()
const route = useRoute()
const entitiesQuery = useMetadataEntitiesQuery()
const grabbed = ref<number | null>(null)
const pointerFrom = ref<number | null>(null)
const pointerMoved = ref(false)

const items = computed(() => navigation.pinnedItems.map((item) => ({
  item,
  path: resolvePath(item),
})))
const visibleItems = computed(() => navigation.pinnedExpanded ? items.value : items.value.slice(0, 5))
const hasMore = computed(() => items.value.length > 5)

function resolvePath(item: PinnedItem): string | null {
  if (item.type === 'page') {
    if (item.app === 'studio' && item.page === 'home') return '/'
    return boot.pages.find(page => page.app === item.app && page.key === item.page)?.path ?? null
  }
  const entity = (entitiesQuery.data.value ?? []).find(candidate => candidate.app.name === item.app && candidate.key === item.entity)
  if (!entity?.slug) return null
  return item.type === 'record' ? `/${entity.slug}/${encodeURIComponent(item.record ?? '')}` : `/${entity.slug}`
}

function move(from: number, to: number) {
  navigation.reorderPinned(from, to)
  grabbed.value = to
}

function onKeydown(event: KeyboardEvent, index: number) {
  if (event.target !== event.currentTarget) return
  if (event.key === ' ' && grabbed.value === null) {
    event.preventDefault()
    grabbed.value = index
  } else if (grabbed.value !== null && (event.key === 'ArrowUp' || event.key === 'ArrowDown')) {
    event.preventDefault()
    move(grabbed.value, Math.max(0, Math.min(items.value.length - 1, grabbed.value + (event.key === 'ArrowUp' ? -1 : 1))))
  } else if (grabbed.value !== null && (event.key === 'Enter' || event.key === ' ')) {
    event.preventDefault()
    grabbed.value = null
  } else if (event.key === 'Escape') {
    grabbed.value = null
  }
}

function onPointerDown(event: PointerEvent, index: number) {
  if (event.pointerType === 'mouse') return
  pointerFrom.value = index
  pointerMoved.value = false
  ;(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId)
}

function onPointerMove(event: PointerEvent) {
  if (pointerFrom.value === null) return
  const target = document.elementFromPoint(event.clientX, event.clientY)?.closest<HTMLElement>('[data-pinned-index]')
  if (!target) return
  const to = Number(target.dataset.pinnedIndex)
  if (Number.isInteger(to) && to !== pointerFrom.value) {
    pointerMoved.value = true
    navigation.reorderPinned(pointerFrom.value, to)
    pointerFrom.value = to
  }
}

function onPointerUp() {
  pointerFrom.value = null
  window.setTimeout(() => { pointerMoved.value = false }, 0)
}

function followPin(event: MouseEvent, path: string | null) {
  if (!path || pointerMoved.value) event.preventDefault()
}
</script>

<template>
  <PopoverRoot v-if="items.length > 0 && props.collapsed">
    <PopoverTrigger as-child>
      <button class="pinned-nav__collapsed" type="button" aria-label="Open pinned items" title="Pinned">
        <Pin :size="16" :stroke-width="1.8" aria-hidden="true" />
      </button>
    </PopoverTrigger>
    <PopoverPortal>
      <PopoverContent class="pinned-nav__popover" side="right" align="start" :side-offset="8">
        <p class="pinned-nav__popover-title">Pinned</p>
        <div class="pinned-nav__popover-list">
          <div v-for="entry in items" :key="entry.item.path" class="pinned-nav__item" :class="{ 'pinned-nav__item--current': studioPinIsCurrent(entry.item.type, entry.path, route.path) }">
            <button class="pinned-nav__remove" type="button" :aria-label="`Unpin ${entry.item.label}`" @click="navigation.unpin(entry.item)">
              <Pin :size="16" :stroke-width="1.8" aria-hidden="true" />
            </button>
            <RouterLink class="pinned-nav__link" :to="entry.path ?? route.fullPath" :aria-disabled="!entry.path" @click="followPin($event, entry.path)">
              <span>{{ entry.item.label }}</span>
            </RouterLink>
          </div>
        </div>
      </PopoverContent>
    </PopoverPortal>
  </PopoverRoot>

  <section v-else-if="items.length > 0" class="pinned-nav" aria-label="Pinned">
    <button class="pinned-nav__heading" type="button" :aria-expanded="navigation.pinnedOpen" @click="navigation.setPinnedOpen(!navigation.pinnedOpen)">
      <span>Pinned</span>
      <ChevronUp v-if="navigation.pinnedOpen" :size="15" aria-hidden="true" />
      <ChevronDown v-else :size="15" aria-hidden="true" />
    </button>
    <div v-if="navigation.pinnedOpen" class="pinned-nav__list">
      <div
        v-for="(entry, index) in visibleItems"
        :key="`${entry.item.type}:${entry.item.app}:${entry.item.entity ?? entry.item.page}:${entry.item.record ?? ''}`"
        class="pinned-nav__item"
        :class="{ 'pinned-nav__item--current': studioPinIsCurrent(entry.item.type, entry.path, route.path), 'pinned-nav__item--grabbed': grabbed === index }"
        :data-pinned-index="index"
        draggable="true"
        tabindex="0"
        :aria-label="`${entry.item.label}. Press Space to reorder.`"
        :aria-pressed="grabbed === index"
        @keydown="onKeydown($event, index)"
        @dragstart="$event.dataTransfer?.setData('text/plain', String(index))"
        @dragover.prevent
        @drop.prevent="navigation.reorderPinned(Number($event.dataTransfer?.getData('text/plain')), index)"
        @pointerdown="onPointerDown($event, index)"
        @pointermove="onPointerMove"
        @pointerup="onPointerUp"
        @pointercancel="onPointerUp"
      >
        <button class="pinned-nav__remove" type="button" :aria-label="`Unpin ${entry.item.label}`" @click="navigation.unpin(entry.item)">
          <Pin :size="16" :stroke-width="1.8" aria-hidden="true" />
        </button>
        <RouterLink
          class="pinned-nav__link"
          :class="{ 'pinned-nav__link--disabled': !entry.path }"
          :to="entry.path ?? route.fullPath"
          :aria-disabled="!entry.path"
          :title="entry.path ? entry.item.label : `${entry.item.label} is unavailable`"
          @click="followPin($event, entry.path)"
        >
          <span>{{ entry.item.label }}</span>
        </RouterLink>
      </div>
      <button v-if="hasMore" class="pinned-nav__more" type="button" @click="navigation.setPinnedExpanded(!navigation.pinnedExpanded)">
        <Ellipsis :size="16" aria-hidden="true" />
        {{ navigation.pinnedExpanded ? 'See less' : 'See more' }}
      </button>
    </div>
  </section>
</template>

<style scoped>
.pinned-nav { display: grid; gap: 4px; width: 100%; margin-bottom: 12px; }
.pinned-nav__heading { display: flex; min-height: 30px; align-items: center; justify-content: space-between; border: 0; background: transparent; color: var(--studio-text-muted); padding: 0 10px; font-size: 12px; font-weight: 500; }
.pinned-nav__list, .pinned-nav__popover-list { display: grid; gap: 4px; }
.pinned-nav__list { max-height: min(45vh, 360px); overflow-y: auto; }
.pinned-nav__item { display: grid; grid-template-columns: 35px minmax(0, 1fr); align-items: center; border-radius: var(--studio-radius-control); touch-action: pan-y; }
.pinned-nav__item:hover, .pinned-nav__item--current { background: var(--studio-surface-raised); }
.pinned-nav__item--grabbed { outline: 2px solid var(--studio-focus); }
.pinned-nav__item:focus-visible { outline: 2px solid var(--studio-focus); outline-offset: 1px; }
.pinned-nav__link { display: flex; min-width: 0; min-height: 34px; align-items: center; color: var(--studio-text-muted); padding: 0 10px 0 0; text-decoration: none; font-size: 13px; font-weight: 600; line-height: 1; }
.pinned-nav__item:hover .pinned-nav__link, .pinned-nav__item--current .pinned-nav__link { color: var(--studio-text); }
.pinned-nav__link span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pinned-nav__link--disabled { opacity: .55; }
.pinned-nav__remove, .pinned-nav__collapsed { display: inline-flex; width: 30px; height: 30px; align-items: center; justify-content: center; border: 0; border-radius: var(--studio-radius-control); background: transparent; color: var(--studio-text-muted); }
.pinned-nav__remove:hover, .pinned-nav__collapsed:hover { background: var(--studio-surface); color: var(--studio-text); }
.pinned-nav__remove { width: 35px; height: 34px; padding: 0 9px 0 10px; }
.pinned-nav :is(button, a):focus-visible, .pinned-nav__popover :is(button, a):focus-visible, .pinned-nav__collapsed:focus-visible { outline: 2px solid var(--studio-focus); outline-offset: -2px; }
.pinned-nav__more { display: flex; min-height: 32px; align-items: center; gap: 9px; border: 0; border-radius: var(--studio-radius-control); background: transparent; color: var(--studio-text-muted); padding: 0 10px; font-size: 13px; font-weight: 600; }
.pinned-nav__more:hover { background: var(--studio-surface-raised); color: var(--studio-text); }
.pinned-nav__collapsed { flex: 0 0 auto; }
.pinned-nav__popover { z-index: 50; width: 240px; max-height: min(420px, 70vh); overflow: auto; border: 1px solid var(--studio-border); border-radius: var(--studio-radius-control); background: var(--studio-surface); box-shadow: var(--studio-shadow-control); padding: 8px; }
.pinned-nav__popover-title { margin: 3px 10px 7px; color: var(--studio-text-muted); font-size: 12px; font-weight: 500; }
@media (max-width: 720px) { .pinned-nav { min-width: 220px; margin: 0; } }
</style>
