import { defineConfig, devices } from '@playwright/test'

// The default page, served by the real server on rendered sample values (serve.sh). Chromium locally, three engines in CI.
// The port serve.sh listens on; the config and the script must agree, so it is fixed here and passed through PORT there.
const PORT = 8089
// CI (or E2E_BROWSERS=all) runs all three engines; a local run uses Chromium for speed.
const all = process.env.E2E_BROWSERS === 'all' || !!process.env.CI

// The suite configuration: one spec directory, parallel files, no leftover test.only on CI.
export default defineConfig({
  testDir: '.',
  // Only *.spec.ts files are tests; serve.sh and the config are not.
  testMatch: '*.spec.ts',
  fullyParallel: true,
  // A committed test.only would silently skip the rest, so CI refuses it.
  forbidOnly: !!process.env.CI,
  // One line per test: short enough to read in a CI log.
  reporter: 'list',
  // Tests address the page by path; the base URL is the server serve.sh starts.
  use: { baseURL: `http://localhost:${PORT}` },
  // Start the real server (not a mock) on rendered sample values, wait for /healthz, and reuse a running one locally.
  webServer: { command: './serve.sh', url: `http://localhost:${PORT}/healthz`, reuseExistingServer: !process.env.CI, timeout: 120_000 },
  // One project per engine; the Firefox and WebKit projects are added only when all browsers are requested.
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
