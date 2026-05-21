import { createApp } from 'vue'

import App from './App.vue'
import { router } from '@/router'
import { pinia } from '@/stores/pinia'
import '@/styles/base.css'

createApp(App).use(pinia).use(router).mount('#app')
