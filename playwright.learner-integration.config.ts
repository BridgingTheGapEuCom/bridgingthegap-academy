import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: 'frontend/e2e',
  testMatch: '**/learner.integration.spec.ts',
  workers: 1,
  use: {
    baseURL: process.env.BTG_LEARNER_E2E_BASE_URL ?? 'http://localhost:5173',
    browserName: 'chromium',
  },
})
