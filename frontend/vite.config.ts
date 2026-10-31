import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          // xterm + addons: ~310KB, rarely change → long-lived cache
          'xterm': [
            '@xterm/xterm',
            '@xterm/addon-fit',
            '@xterm/addon-search',
            '@xterm/addon-web-links',
          ],
          // Vue runtime + Pinia: ~85KB, rarely change
          'vue-vendor': ['vue', 'pinia'],
        },
      },
    },
  },
})
