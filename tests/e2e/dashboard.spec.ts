import { expect, test } from "@orieken/saturday-playwright";
import type { Page } from "@playwright/test";

import { dashboardUrl, serviceUrls } from "./config.js";

const transactionTypes = ["834", "820", "270", "271", "275", "278", "837", "835", "276", "277", "269", "999", "277CA"] as const;

type DemoTransactionType = (typeof transactionTypes)[number];

type DemoTransaction = {
  id: string;
  type: DemoTransactionType;
  status: string;
  senderId: string;
  receiverId: string;
  payload: Record<string, string>;
  rawX12: string;
  relatedId?: string;
  createdAt: string;
};

const demoTransactions: DemoTransaction[] = transactionTypes.map((type, index) => ({
  id: `tx-e2e-${type.toLowerCase()}`,
  type,
  status: type === "999" ? "Accepted" : "Dispatched",
  senderId: type === "834" || type === "820" || type === "999" || type === "277CA" ? "Adventure Society" : "provider-vitesse-temple",
  receiverId: type === "834" || type === "820" || type === "999" || type === "277CA" ? "provider-vitesse-temple" : "Adventure Society",
  payload: {
    x12: `${type} dashboard display fixture`,
    claimId: `claim-e2e-${type.toLowerCase()}`,
    adventurerId: "adv-e2e-dashboard",
    ...(type === "275" ? { attachmentType: "OZ", reportTypeCode: "B4" } : {})
  },
  rawX12: `ISA*00*          *00*          *ZZ*ASHN           *ZZ*PARTNER        *260708*1200*^*00501*${String(index + 1).padStart(9, "0")}*0*T*:~ST*${type}*0001~SE*2*0001~`,
  relatedId: index > 0 ? "tx-e2e-834" : undefined,
  createdAt: new Date(Date.UTC(2026, 6, 8, 12, index, 0)).toISOString()
}));

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

    await expect(page.getByLabel("Transaction type")).toContainText("275");
    await page.getByLabel("Transaction type").selectOption("275");
    await expect(page.getByLabel("Transaction type")).toHaveValue("275");

    await page.getByRole("button", { name: /Partners/i }).click();
    await expect(page.getByRole("heading", { name: /Trading Partners/i })).toBeVisible();

    expect(consoleLogger.getLogs()).toEqual(expect.any(Array));
  });

  test("displays data for every supported transaction type filter", async ({ page }) => {
    await mockDashboardApi(page);
    await page.goto(dashboardUrl);

    await page.getByRole("button", { name: /Ledger/i }).click();

    for (const type of transactionTypes) {
      const transaction = demoTransactions.find((item) => item.type === type);
      expect(transaction).toBeTruthy();

      await page.getByLabel("Transaction type").selectOption(type);
      await expect(page.getByText(transaction!.id)).toBeVisible();
      await expect(page.getByText(`${type} · ${transaction!.status}`)).toBeVisible();

      await page.getByText(transaction!.id).click();
      await expect(page.getByRole("heading", { name: /Transaction Detail/i })).toBeVisible();
      await expect(page.getByText(transaction!.rawX12)).toBeVisible();
      await expect(page.getByText(`${type} dashboard display fixture`)).toBeVisible();
      await page.getByRole("button", { name: "Close" }).click();
    }
  });

  test("labels 275 claim attachments inside the transaction timeline", async ({ page }) => {
    await mockDashboardApi(page);
    await page.goto(dashboardUrl);

    await page.getByRole("button", { name: /Timeline/i }).click();
    await page.getByLabel("Transaction type").selectOption("275");

    await expect(page.getByText("Claim lifecycle")).toBeVisible();
    await expect(page.getByText("Claim claim-e2e-275")).toBeVisible();
    await expect(page.getByRole("button", { name: /275 OZ\/B4 attachment/i })).toBeVisible();
  });

  test("supports manual approval for a pending 278 authorization", async ({ page }) => {
    await mockDashboardApi(page);
    await page.goto(dashboardUrl);

    await page.getByRole("button", { name: /Send 834 Enrollment/i }).click();
    await expect(page.locator(".result").filter({ hasText: "Filter Fixture Ranger" })).toBeVisible();

    await page.getByRole("button", { name: /278 Resurrection Auth/i }).click();
    const authReview = page.locator(".auth-review-card");
    await expect(authReview.getByText("Prior Auth Review")).toBeVisible();
    await expect(authReview.getByText("278 · Pending")).toBeVisible();

    await page.getByRole("button", { name: /Approve Auth/i }).click();
    await expect(authReview.getByText("278 · Approved")).toBeVisible();
  });
});

async function mockDashboardApi(page: Page) {
  await page.route("**/v1/**", async (route) => {
    const url = new URL(route.request().url());
    const path = url.pathname;

    if (path === "/v1/health") {
      await route.fulfill({ json: { data: { "api-gateway": "ok", "payer-core": "ok", "provider-service": "ok", "edi-intake": "ok" } } });
      return;
    }

    if (path === "/v1/providers") {
      await route.fulfill({
        json: {
          data: [
            { id: "provider-vitesse-temple", name: "Temple of the Healer, Vitesse", providerType: "temple", tierRank: "gold", region: "Vitesse" }
          ]
        }
      });
      return;
    }

    if (path === "/v1/x12/trading-partners") {
      await route.fulfill({
        json: {
          data: [
            {
              id: "tp-vitesse-temple",
              name: "Temple of the Healer, Vitesse",
              senderId: "provider-vitesse-temple",
              receiverId: "Adventure Society",
              allowedTransactionTypes: [...transactionTypes],
              routeTarget: "payer-core",
              status: "active"
            }
          ]
        }
      });
      return;
    }

    if (path === "/v1/adventurers" && route.request().method() === "POST") {
      await route.fulfill({
        status: 201,
        json: {
          data: { id: "adv-e2e-dashboard", name: "Filter Fixture Ranger", rank: "Gold", guild: "E2E Guild", region: "Vitesse", coverageStatus: "Active" },
          transaction: demoTransactions.find((transaction) => transaction.type === "834")
        }
      });
      return;
    }

    if (path === "/v1/adventurers") {
      await route.fulfill({
        json: {
          data: [
            { id: "adv-e2e-dashboard", name: "Filter Fixture Ranger", rank: "Gold", guild: "E2E Guild", region: "Vitesse", coverageStatus: "Active" }
          ],
          page: pageInfo(1, 10)
        }
      });
      return;
    }

    if (path === "/v1/auth-requests") {
      await route.fulfill({
        status: 202,
        json: {
          data: { authorizationStatus: "Pending", serviceType: "resurrection", incidentSeverity: "Diamond", review: "queued" },
          transaction: {
            ...demoTransactions.find((transaction) => transaction.type === "278"),
            id: "tx-e2e-auth-review",
            status: "Pending"
          }
        }
      });
      return;
    }

    if (path === "/v1/auth-requests/tx-e2e-auth-review/decision") {
      await route.fulfill({
        json: {
          data: { authorizationStatus: "Approved", transactionId: "tx-e2e-auth-review", reason: "manual approval" },
          transaction: {
            ...demoTransactions.find((transaction) => transaction.type === "278"),
            id: "tx-e2e-auth-review",
            status: "Approved"
          }
        }
      });
      return;
    }

    if (path === "/v1/claims") {
      await route.fulfill({
        json: {
          data: [
            {
              id: "claim-e2e-dashboard",
              adventurerId: "adv-e2e-dashboard",
              providerId: "provider-vitesse-temple",
              incidentSeverity: "fixture",
              transactionId: "tx-e2e-837",
              amountCents: 12500,
              status: "Submitted"
            }
          ],
          page: pageInfo(1, 10)
        }
      });
      return;
    }

    if (path === "/v1/transactions") {
      const type = url.searchParams.get("type");
      const data = type ? demoTransactions.filter((transaction) => transaction.type === type) : demoTransactions;
      await route.fulfill({ json: { data, page: pageInfo(data.length, 25) } });
      return;
    }

    const transactionMatch = path.match(/^\/v1\/transactions\/([^/]+)$/);
    if (transactionMatch) {
      const transaction = demoTransactions.find((item) => item.id === transactionMatch[1]);
      await route.fulfill({ status: transaction ? 200 : 404, json: transaction ? { data: transaction } : { error: "transaction not found" } });
      return;
    }

    if (path === "/v1/x12/messages") {
      await route.fulfill({ json: { data: [], page: pageInfo(0, 10) } });
      return;
    }

    await route.fulfill({ status: 404, json: { error: `unmocked route ${path}` } });
  });
}

function pageInfo(count: number, limit: number) {
  return {
    limit,
    offset: 0,
    count,
    hasMore: false
  };
}
