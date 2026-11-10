import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// Development equivalent of the production proxy's X-Accel-Redirect.
function sharedListRouting() {
  return {
    name: 'shared-list-routing',
    configureServer(server) {
      server.middlewares.use(async (req, res, next) => {
        const url = new URL(req.url, 'http://localhost')
        if (/^\/reservation\/?$/.test(url.pathname)) {
          const writeHead = res.writeHead
          res.writeHead = function (...args) {
            this.setHeader('Cache-Control', 'no-store')
            this.setHeader('Referrer-Policy', 'no-referrer')
            this.setHeader('X-Robots-Tag', 'noindex, nofollow, noarchive')
            return writeHead.apply(this, args)
          }
          return next()
        }
        if (!['GET', 'HEAD'].includes(req.method) || !(/^\/[A-Za-z0-9_]{3,30}\/wishlist\/shared\/?$/.test(url.pathname) || /^\/u\/[A-Za-z0-9_]{3,30}\/wishlist\/[^/]+(?:\/[^/]+)?\/?$/.test(url.pathname))) return next()
        try {
          const target = new URL(req.url, process.env.API_PROXY_TARGET || 'http://server:8080')
          const response = await fetch(target, { method: req.method, redirect: 'manual', signal: AbortSignal.timeout(5000) })
          for (const name of ['location', 'cache-control', 'referrer-policy', 'x-robots-tag']) {
            const value = response.headers.get(name)
            if (value) res.setHeader(name, value)
          }
          if (response.headers.has('x-accel-redirect')) return next()
          res.statusCode = response.status
          res.end()
        } catch {
          res.statusCode = 503
          res.end()
        }
      })
    },
  }
}

export default defineConfig({
  plugins: [sharedListRouting(), vue()],
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

