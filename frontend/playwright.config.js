import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  outputDir: 'test-results/browser',
  fullyParallel: false,
  workers: 1,
  retries: 0,
  forbidOnly: Boolean(process.env.CI),
  reporter: [['list'], ['html', { open: 'never' }]],
  use: {
    baseURL: 'http://127.0.0.1:4173',
    channel: process.env.PLAYWRIGHT_CHANNEL || (process.platform === 'darwin' ? 'chrome' : undefined),
    headless: true, viewport: { width: 1120, height: 900 },
    trace: 'retain-on-failure', screenshot: 'only-on-failure',
  },
  webServer: [
    { name: 'Go harness', command: 'node ../scripts/run.mjs harness', url: 'http://127.0.0.1:4317/health', reuseExistingServer: false, timeout: 120000, gracefulShutdown: { signal: 'SIGTERM', timeout: 5000 } },
    { name: 'Vue UI', command: 'npm run dev -- --mode harness', url: 'http://127.0.0.1:4173', reuseExistingServer: false, gracefulShutdown: { signal: 'SIGTERM', timeout: 5000 } },
  ],
})
