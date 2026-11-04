import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    watch: {
      // File changes in Windows bind mounts must also trigger HMR.
      usePolling: true,
    },
    proxy: {
      '/api': {
        target: process.env.API_PROXY_TARGET || 'http://server:8080',
        configure(proxy) {
          proxy.on('proxyReq', (proxyRequest, request) => {
            proxyRequest.setHeader('X-Real-IP', request.socket.remoteAddress)
            proxyRequest.removeHeader('X-Forwarded-For')
          })
        },
      },
    },
  },
})

