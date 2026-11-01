import { defineConfig, devices } from '@playwright/test'

// The default page, served by the real server on rendered sample values (serve.sh). Chromium locally, three engines in CI.
const PORT = 8089
const all = process.env.E2E_BROWSERS === 'all' || !!process.env.CI

export default defineConfig({
  testDir: '.',
  testMatch: '*.spec.ts',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  reporter: 'list',
  use: { baseURL: `http://localhost:${PORT}` },
  webServer: { command: './serve.sh', url: `http://localhost:${PORT}/healthz`, reuseExistingServer: !process.env.CI, timeout: 120_000 },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
    ...(all
      ? [
          { name: 'firefox', use: { ...devices['Desktop Firefox'] } },
          { name: 'webkit', use: { ...devices['Desktop Safari'] } },
        ]
      : []),
  ],
})
