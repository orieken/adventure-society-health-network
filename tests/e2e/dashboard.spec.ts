import { expect, test } from "@orieken/saturday-playwright";

import { dashboardUrl, serviceUrls } from "./config.js";

test.describe("ASHN dashboard smoke", () => {
  test.skip(!dashboardUrl, "Set ASHN_DASHBOARD_URL to run dashboard browser smoke tests.");

  test("renders the RPG dashboard shell and service links", async ({ consoleLogger, page }) => {
    await page.goto(dashboardUrl);

    await expect(page.getByRole("heading", { name: /ASHN Transaction Dashboard/i })).toBeVisible();
    await expect(page.getByText("Adventure Society Health Network")).toBeVisible();
    await expect(page.getByRole("link", { name: serviceUrls.apiGateway })).toHaveAttribute("href", serviceUrls.apiGateway);

    await expect(page.getByRole("button", { name: /Workflow/i })).toBeVisible();
    await expect(page.getByRole("button", { name: /Timeline/i })).toBeVisible();
    await expect(page.getByRole("button", { name: /Ledger/i })).toBeVisible();
    await expect(page.getByRole("button", { name: /XML Intake/i })).toBeVisible();
    await expect(page.getByRole("button", { name: /Partners/i })).toBeVisible();

    await page.getByRole("button", { name: /Partners/i }).click();
    await expect(page.getByRole("heading", { name: /Trading Partners/i })).toBeVisible();

    expect(consoleLogger.getLogs()).toEqual(expect.any(Array));
  });
});
