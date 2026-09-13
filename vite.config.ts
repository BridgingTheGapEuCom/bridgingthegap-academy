import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  root: 'frontend',
  build: { outDir: '../internal/web/dist', emptyOutDir: true },
  server: { proxy: { '/health': 'http://localhost:8080' } },
  test: { environment: 'jsdom', include: ['src/**/*.test.ts'] },
})
