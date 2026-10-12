import { defineConfig } from 'vitest/config'
import { loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig(({ mode }) => {
  const backendOrigin = loadEnv(mode, '.', 'BTG_').BTG_LMS_DEV_BACKEND_ORIGIN ?? 'http://127.0.0.1:8080'
  return {
    plugins: [vue()],
    root: 'frontend',
    build: { outDir: '../internal/web/dist', emptyOutDir: true },
    server: { proxy: { '/health': backendOrigin, '/api': backendOrigin } },
    test: { environment: 'jsdom', include: ['src/**/*.test.ts'] },
  }
})
