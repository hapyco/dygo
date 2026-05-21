<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  name?: string
  imageUrl?: string
  size?: 'sm' | 'md'
}>(), {
  name: 'Studio user',
  size: 'md',
})

const initials = computed(() => {
  const words = props.name.trim().split(/\s+/).filter(Boolean)
  if (words.length === 0) {
    return 'SU'
  }

  return words.slice(0, 2).map((word) => word[0]?.toUpperCase()).join('')
})
</script>

<template>
  <span class="d-avatar" :data-size="size" :aria-label="name">
    <img v-if="imageUrl" class="d-avatar__image" :src="imageUrl" :alt="name" />
    <span v-else class="d-avatar__fallback" aria-hidden="true">{{ initials }}</span>
  </span>
</template>

<style scoped>
.d-avatar {
  display: inline-flex;
  width: 32px;
  height: 32px;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border: 1px solid var(--studio-border);
  border-radius: 999px;
  background:
    radial-gradient(circle at 32% 24%, oklch(0.88 0.036 74), transparent 32%),
    linear-gradient(135deg, oklch(0.42 0.035 246), oklch(0.25 0.03 246));
  color: oklch(0.99 0.004 246);
  box-shadow: var(--studio-shadow-control);
}

.d-avatar[data-size='sm'] {
  width: 28px;
  height: 28px;
}

.d-avatar__image {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.d-avatar__fallback {
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0;
  line-height: 1;
}

.d-avatar[data-size='sm'] .d-avatar__fallback {
  font-size: 11px;
}
</style>
