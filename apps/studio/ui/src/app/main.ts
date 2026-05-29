import { createApp } from 'vue'
import { VueQueryPlugin } from '@tanstack/vue-query'

import App from './App.vue'
import { queryClient } from './query'
import { router } from '@/router'
import { pinia } from '@/stores/pinia'
import '@/styles/base.css'

createApp(App).use(pinia).use(VueQueryPlugin, { queryClient }).use(router).mount('#app')
