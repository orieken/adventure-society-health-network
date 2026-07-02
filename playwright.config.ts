import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "tests/e2e",
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? [["github"], ["html", { open: "never" }]] : [["list"], ["html", { open: "never" }]],
  timeout: 30_000,
  expect: {
    timeout: 5_000
  },
  use: {
    trace: "retain-on-failure"
  },
  projects: [
    {
      name: "contracts",
      testMatch: /service-contracts\.spec\.ts/
    },
    {
      name: "dashboard",
      testMatch: /dashboard\.spec\.ts/,
      use: {
        ...devices["Desktop Chrome"]
      }
    }
  ]
});
