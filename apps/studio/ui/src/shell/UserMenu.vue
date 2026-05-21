<script setup lang="ts">
import Avatar from '@/design/atoms/Avatar.vue'

withDefaults(defineProps<{
  userName?: string
  userAvatarUrl?: string
}>(), {
  userName: 'Studio user',
})
</script>

<template>
  <Avatar :name="userName" :image-url="userAvatarUrl" />
</template>
