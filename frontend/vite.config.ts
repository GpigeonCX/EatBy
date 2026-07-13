import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { VitePWA } from 'vite-plugin-pwa'

export default defineConfig({
  plugins: [
    vue(),
    VitePWA({
      registerType: 'autoUpdate',
      includeAssets: ['pantry.svg'],
      manifest: {
        name: '家庭食材', short_name: '食材', description: '轻量家庭食材与保质期管理',
        theme_color: '#245b45', background_color: '#f6f4ed', display: 'standalone',
        icons: [{ src: '/pantry.svg', sizes: 'any', type: 'image/svg+xml', purpose: 'any maskable' }]
      },
      workbox: { navigateFallback: '/index.html' }
    })
  ],
  server: { proxy: { '/api': 'http://localhost:8080', '/healthz': 'http://localhost:8080' } },
  build: { outDir: '../cmd/server/web', emptyOutDir: true }
})
