<script setup lang="ts">
import { Home } from '@lucide/vue'
</script>

<template>
  <nav class="studio-breadcrumbs" aria-label="Breadcrumb">
    <a class="studio-breadcrumbs__home" href="/" aria-label="Home">
      <Home class="studio-breadcrumbs__home-icon" :size="16" :stroke-width="1.8" aria-hidden="true" />
    </a>
  </nav>
</template>

<style scoped>
.studio-breadcrumbs {
  display: inline-flex;
  align-items: center;
}

.studio-breadcrumbs__home {
  display: inline-flex;
  width: 32px;
  height: 32px;
  align-items: center;
  justify-content: center;
  border: 1px solid transparent;
  border-radius: var(--studio-radius-control);
  background: transparent;
  color: var(--studio-text-muted);
  text-decoration: none;
  transition:
    background-color 160ms ease,
    border-color 160ms ease,
    color 160ms ease;
}

.studio-breadcrumbs__home:hover {
  background: var(--studio-surface-raised);
  color: var(--studio-text);
}

.studio-breadcrumbs__home:focus-visible {
  outline: 2px solid var(--studio-focus);
  outline-offset: 2px;
}

.studio-breadcrumbs__home-icon {
  flex: 0 0 auto;
}
</style>
