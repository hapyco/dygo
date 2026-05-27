<script setup lang="ts">
import PageHeader from '@/shell/PageHeader.vue'
</script>

<template>
  <section class="studio-page" aria-labelledby="studio-home-title">
    <!--
      TODO: Home is a Studio-delivered Page, not a separate dashboard outside the
      Page model. It should render inside the normal Shell/PageSheet contract.
    -->
    <!--
      TODO: First version can show readable Entity routes as cards grouped by
      App. Later, explicit Space/Folder metadata can control the grouping.
    -->
    <!--
      TODO: Keep Home route cards typed and permission-aware: entity list,
      space, report, dashboard, settings, custom page, and maybe external link.
    -->
    <PageHeader
      title-id="studio-home-title"
      title="Page sheet"
    />
  </section>
</template>
