import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './native-e2e',
  workers: 1,
  retries: 0,
  timeout: 60000,
  forbidOnly: true,
  reporter: [['list']],
  outputDir: 'test-results/native-focus',
  use: {
    channel: process.env.PLAYWRIGHT_CHANNEL || 'chrome',
    headless: false,
    viewport: { width: 1000, height: 750 },
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
})
