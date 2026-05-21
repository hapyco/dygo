<template>
  <nav class="studio-breadcrumbs" aria-label="Breadcrumb">
    <a class="studio-breadcrumbs__home" href="/" aria-label="Home">
      <span class="studio-breadcrumbs__home-icon" aria-hidden="true" />
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
  position: relative;
  width: 14px;
  height: 12px;
  border: 1.6px solid currentColor;
  border-top: 0;
  border-radius: 2px;
}

.studio-breadcrumbs__home-icon::before {
  position: absolute;
  top: -6px;
  left: 1px;
  width: 9px;
  height: 9px;
  border-left: 1.6px solid currentColor;
  border-top: 1.6px solid currentColor;
  content: '';
  transform: rotate(45deg);
}
</style>
