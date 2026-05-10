import { fileURLToPath, URL } from 'node:url'

import tailwindcss from '@tailwindcss/vite'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
      '@dygo/ui': fileURLToPath(new URL('./src/design/index.ts', import.meta.url)),
      '@dygo/ui/': fileURLToPath(new URL('./src/design/', import.meta.url)),
    },
  },
  server: {
    port: 6791,
    strictPort: true,
    proxy: {
      '/api': 'http://127.0.0.1:6790',
    },
  },
})
