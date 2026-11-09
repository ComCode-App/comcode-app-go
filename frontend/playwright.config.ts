import { defineConfig, devices } from "@playwright/test";

// E2E smoke tests run against an already running stack (Go API + Next.js + Postgres + Redis).
// Locally: `make local-dev`, `make create-super-admin`, then `pnpm e2e`. See docs/DEVELOPMENT.md#e2e.
const baseURL = process.env.E2E_BASE_URL ?? "http://localhost:3000";

export default defineConfig({
  testDir: "./e2e",
  globalSetup: "./e2e/global-setup.ts",
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  timeout: 90_000,
  expect: { timeout: 15_000 },
  reporter: process.env.CI
    ? [["list"], ["html", { open: "never" }]]
    : [["list"]],
  use: {
    baseURL,
    locale: "en-US",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    // First hits compile routes in `next dev`; give navigations room.
    navigationTimeout: 60_000,
    actionTimeout: 15_000,
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
