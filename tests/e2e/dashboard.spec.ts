import { expect, test } from "@orieken/saturday-playwright";
import type { Page } from "@playwright/test";

import { dashboardUrl } from "./config.js";

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
  status: type === "999" || type === "275" ? "Accepted" : "Dispatched",
  senderId: type === "834" || type === "820" || type === "999" || type === "277CA" ? "Adventure Society" : "provider-vitesse-temple",
  receiverId: type === "834" || type === "820" || type === "999" || type === "277CA" ? "provider-vitesse-temple" : "Adventure Society",
  payload: {
    x12: `${type} dashboard display fixture`,
    claimId: `claim-e2e-${type.toLowerCase()}`,
    adventurerId: "adv-e2e-dashboard",
    ...(type === "275"
      ? {
          attachmentType: "OZ",
          reportTypeCode: "B4",
          packetId: "packet-e2e-275",
          packetSequence: "1",
          packetCount: "2",
          attachmentReviewStatus: "Received",
          documentReferenceId: "doc-e2e-275",
          documentReferenceUrl: "https://docs.example.test/doc-e2e-275.pdf"
        }
      : {})
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
    await expect(page.locator(".gateway-url")).toHaveAttribute("href", /^https?:\/\/.+/);

    await expect(page.getByRole("button", { name: /Workflow/i })).toBeVisible();
    await expect(page.getByRole("button", { name: /Timeline/i })).toBeVisible();
    await expect(page.getByRole("button", { name: /Ledger/i })).toBeVisible();
    await expect(page.getByRole("button", { name: /XML Intake/i })).toBeVisible();
    await expect(page.getByRole("button", { name: /Partners/i })).toBeVisible();

    await page.getByRole("button", { name: /Ledger/i }).click();
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
      await expect(page.getByRole("tab", { name: "JSON" })).toHaveAttribute("aria-selected", "true");
      await expect(page.getByText(`${type} dashboard display fixture`)).toBeVisible();

      await page.getByRole("tab", { name: "XML" }).click();
      await expect(page.getByText(`<AshnTransaction id="${transaction!.id}" type="${type}" status="${transaction!.status}">`)).toBeVisible();
      await expect(page.getByText("<PayloadJson>")).toBeVisible();

      await page.getByRole("tab", { name: "X12" }).click();
      await expect(page.getByText(transaction!.rawX12)).toBeVisible();
      await page.getByRole("button", { name: "Close" }).click();
    }
  });

  test("saves applies and deletes dashboard filter presets", async ({ page }) => {
    await mockDashboardApi(page);
    await page.goto(dashboardUrl);

    await page.getByRole("button", { name: /Ledger/i }).click();
    await page.getByLabel("Transaction type").selectOption("275");
    await page.getByLabel("Transaction status").selectOption("Accepted");
    await page.getByLabel("Filter name").fill("Accepted 275s");
    await page.getByRole("button", { name: "Save Filter" }).click();

    await page.getByRole("button", { name: "Clear" }).click();
    await expect(page.getByLabel("Transaction type")).toHaveValue("All");
    await page.getByLabel("Saved filters").selectOption({ label: "Accepted 275s" });

    await expect(page.getByLabel("Transaction type")).toHaveValue("275");
    await expect(page.getByLabel("Transaction status")).toHaveValue("Accepted");

    await page.getByRole("button", { name: "Delete Saved" }).click();
    await expect(page.getByLabel("Saved filters")).not.toContainText("Accepted 275s");
  });

  test("labels 275 claim attachments inside the transaction timeline", async ({ page }) => {
    await mockDashboardApi(page);
    await page.goto(dashboardUrl);

    await page.getByRole("button", { name: /Timeline/i }).click();
    await page.getByLabel("Transaction type").selectOption("275");

    await expect(page.getByText("Claim lifecycle")).toBeVisible();
    await expect(page.getByText("Claim claim-e2e-275")).toBeVisible();
    await expect(page.getByRole("button", { name: /275 OZ\/B4 attachment/i })).toBeVisible();
    await expect(page.getByRole("button", { name: /packet-e2e-275 \(1\/2\)/i })).toBeVisible();
  });

  test("manages trading partner profiles", async ({ page }) => {
    await mockDashboardApi(page);
    await page.goto(dashboardUrl);

    await page.getByRole("button", { name: /Partners/i }).click();
    await page.getByLabel("Partner name").fill("Crystal Tower Partner");
    await page.getByLabel("Sender ID").fill("provider-crystal-tower");
    await page.getByLabel("Receiver ID").fill("Adventure Society");
    await page.getByLabel("Allowed X12 types").fill("270,275,837");
    await page.getByRole("button", { name: /Create Partner/i }).click();

    await expect(page.getByText("Crystal Tower Partner")).toBeVisible();
    const crystalCard = page.locator(".partner-card").filter({ hasText: "Crystal Tower Partner" });
    await crystalCard.getByRole("button", { name: "Edit" }).click();
    await page.getByLabel("Partner name").fill("Crystal Tower Updated");
    await page.getByRole("button", { name: /Update Partner/i }).click();

    await expect(page.getByText("Crystal Tower Updated")).toBeVisible();
    const updatedCard = page.locator(".partner-card").filter({ hasText: "Crystal Tower Updated" });
    await updatedCard.getByRole("button", { name: "Delete" }).click();
    await expect(page.getByText("Crystal Tower Updated")).not.toBeVisible();
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

  test("shows async worker queue status transitions", async ({ page }) => {
    await mockDashboardApi(page);
    await page.goto(dashboardUrl);

    const queue = page.locator(".panel").filter({ has: page.getByRole("heading", { name: /Async Worker Queue/i }) });
    await expect(queue.getByText("auth_review · pending")).toBeVisible();
    await expect(queue.getByText("claim_finalization · failed · Dead Letter")).toBeVisible();
    await expect(queue.getByRole("button", { name: /Replay/i })).toBeVisible();
  });

  test("sends 275 documentation for a pending 278 authorization", async ({ page }) => {
    await mockDashboardApi(page);
    await page.goto(dashboardUrl);

    await page.getByRole("button", { name: /Send 834 Enrollment/i }).click();
    await page.getByRole("button", { name: /278 Resurrection Auth/i }).click();

    const authReview = page.locator(".auth-review-card");
    await expect(authReview.getByText("278 · Pending")).toBeVisible();

    await authReview.getByRole("button", { name: /Send 275 Auth Docs/i }).click();
    const latestEvent = page.locator(".event").first();
    await expect(latestEvent.getByText("275 · Accepted")).toBeVisible();
    await latestEvent.getByText("Raw payload").click();
    await expect(latestEvent.getByText("authorizationTransactionId")).toBeVisible();
  });

  test("requests 275 documentation from claim detail", async ({ page }) => {
    await mockDashboardApi(page);
    await page.goto(dashboardUrl);

    await page.getByRole("button", { name: /Ledger/i }).click();
    await page.getByText("claim-e2e-dashboard").click();
    const drawer = page.getByLabel("Selected record details");
    await expect(drawer.getByRole("heading", { name: /Claim Detail/i })).toBeVisible();
    await expect(drawer.getByText("Submitted")).toBeVisible();
    await expect(drawer.locator(".detail-item").filter({ hasText: "Prior Auth" }).getByText("tx-e2e-auth-review")).toBeVisible();
    await expect(drawer.locator(".detail-item").filter({ hasText: "Auth Status" }).getByText("Approved")).toBeVisible();

    await page.getByRole("button", { name: /Request 275 Docs/i }).click();
    await expect(drawer.getByText("Pending Documentation")).toBeVisible();
  });

  test("reviews 275 attachment outcomes separately from EDI acceptance", async ({ page }) => {
    await mockDashboardApi(page);
    await page.goto(dashboardUrl);

    await page.getByRole("button", { name: /Ledger/i }).click();
    await page.getByLabel("Transaction type").selectOption("275");
    await page.getByText("tx-e2e-275").click();

    const drawer = page.getByLabel("Selected record details");
    await expect(drawer.getByRole("heading", { name: /Transaction Detail/i })).toBeVisible();
    const reviewRow = drawer.locator(".detail-item").filter({ hasText: "Attachment Review" });
    await expect(reviewRow.getByText("Received", { exact: true })).toBeVisible();
    await expect(drawer.locator(".detail-item").filter({ hasText: "Document Ref" }).getByText("doc-e2e-275")).toBeVisible();
    await expect(drawer.locator(".detail-item").filter({ hasText: "Document URL" }).getByText("https://docs.example.test/doc-e2e-275.pdf")).toBeVisible();

    await drawer.getByRole("button", { name: /Reject Attachment/i }).click();
    await expect(reviewRow.getByText("Rejected", { exact: true })).toBeVisible();
    await expect(drawer.locator(".detail-item").filter({ hasText: "Review Reason" }).getByText("Supporting documentation is insufficient for business review.")).toBeVisible();
    await expect(drawer.getByText("Accepted")).toBeVisible();
  });
});

async function mockDashboardApi(page: Page) {
  let claimStatus = "Submitted";
  const partnerProfiles = [
    {
      id: "tp-vitesse-temple",
      name: "Temple of the Healer, Vitesse",
      senderId: "provider-vitesse-temple",
      receiverId: "Adventure Society",
      allowedTransactionTypes: [...transactionTypes],
      routeTarget: "payer-core",
      status: "active"
    }
  ];
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

    if (path === "/v1/x12/trading-partners" && route.request().method() === "POST") {
      const partner = route.request().postDataJSON() as typeof partnerProfiles[number];
      const id = partner.id || `tp-${partner.senderId}`;
      const saved = { ...partner, id };
      partnerProfiles.push(saved);
      await route.fulfill({ status: 201, json: { data: saved, lore: "Trading partner profile saved for routing." } });
      return;
    }

    if (path.startsWith("/v1/x12/trading-partners/") && route.request().method() === "PUT") {
      const id = decodeURIComponent(path.split("/").pop() ?? "");
      const partner = route.request().postDataJSON() as typeof partnerProfiles[number];
      const index = partnerProfiles.findIndex((item) => item.id === id);
      const saved = { ...partner, id };
      if (index >= 0) partnerProfiles[index] = saved;
      await route.fulfill({ json: { data: saved, lore: "Trading partner profile saved for routing." } });
      return;
    }

    if (path.startsWith("/v1/x12/trading-partners/") && route.request().method() === "DELETE") {
      const id = decodeURIComponent(path.split("/").pop() ?? "");
      const index = partnerProfiles.findIndex((item) => item.id === id);
      if (index >= 0) partnerProfiles.splice(index, 1);
      await route.fulfill({ json: { data: { id }, lore: "Trading partner profile removed from routing." } });
      return;
    }

    if (path === "/v1/x12/trading-partners") {
      await route.fulfill({
        json: {
          data: partnerProfiles
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

    if (path === "/v1/auth-requests/tx-e2e-auth-review/attachments") {
      await route.fulfill({
        status: 201,
        json: {
          data: { authorizationTransactionId: "tx-e2e-auth-review", attachmentType: "OZ", attachmentControlNumber: "ATTACH-TX-E2E" },
          lore: "Patient information attachment accepted for prior authorization.",
          transaction: {
            ...demoTransactions.find((transaction) => transaction.type === "275"),
            id: "tx-e2e-auth-275",
            status: "Accepted",
            relatedId: "tx-e2e-auth-review",
            payload: {
              x12: "275 dashboard auth attachment fixture",
              authorizationTransactionId: "tx-e2e-auth-review",
              adventurerId: "adv-e2e-dashboard",
              attachmentType: "OZ",
              reportTypeCode: "B4"
            }
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
              authorizationTransactionId: "tx-e2e-auth-review",
              authorizationStatus: "Approved",
              authorizationReason: "Manual review approved resurrection medical necessity.",
              amountCents: 12500,
              status: claimStatus
            }
          ],
          page: pageInfo(1, 10)
        }
      });
      return;
    }

    if (path === "/v1/claims/claim-e2e-dashboard") {
      await route.fulfill({
        json: {
          data: {
            id: "claim-e2e-dashboard",
            adventurerId: "adv-e2e-dashboard",
            providerId: "provider-vitesse-temple",
            incidentSeverity: "fixture",
            transactionId: "tx-e2e-837",
            authorizationTransactionId: "tx-e2e-auth-review",
            authorizationStatus: "Approved",
            authorizationReason: "Manual review approved resurrection medical necessity.",
            amountCents: 12500,
            status: claimStatus
          }
        }
      });
      return;
    }

    if (path === "/v1/claims/claim-e2e-dashboard/documentation-request") {
      claimStatus = "Pending Documentation";
      await route.fulfill({
        status: 202,
        json: {
          data: { claimId: "claim-e2e-dashboard", status: claimStatus, requestedTransaction: "275" },
          transaction: {
            ...demoTransactions.find((transaction) => transaction.type === "277"),
            id: "tx-e2e-doc-request",
            relatedId: "tx-e2e-837"
          }
        }
      });
      return;
    }

    if (path === "/v1/transactions/tx-e2e-275/attachment-review") {
      await route.fulfill({
        json: {
          data: { transactionId: "tx-e2e-275", attachmentReviewStatus: "Rejected", reason: "Supporting documentation is insufficient for business review." },
          lore: "Attachment review marked rejected.",
          transaction: {
            ...demoTransactions.find((transaction) => transaction.id === "tx-e2e-275"),
            payload: {
              ...demoTransactions.find((transaction) => transaction.id === "tx-e2e-275")?.payload,
              attachmentReviewStatus: "Rejected",
              attachmentReviewReason: "Supporting documentation is insufficient for business review."
            }
          }
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

    if (path === "/v1/jobs") {
      await route.fulfill({
        json: {
          data: [
            {
              id: "job-e2e-auth",
              type: "auth_review",
              entityId: "tx-e2e-auth-review",
              status: "pending",
              attempts: 0,
              runAfter: "2026-07-09T15:00:00Z",
              deadLetter: false,
              createdAt: "2026-07-09T15:00:00Z",
              updatedAt: "2026-07-09T15:00:00Z"
            },
            {
              id: "job-e2e-dead",
              type: "claim_finalization",
              entityId: "claim-e2e-dashboard",
              status: "failed",
              attempts: 3,
              runAfter: "2026-07-09T15:01:00Z",
              lastError: "{\"error\":\"simulated adjudication failure\"}",
              deadLetter: true,
              createdAt: "2026-07-09T15:00:00Z",
              updatedAt: "2026-07-09T15:01:00Z"
            }
          ]
        }
      });
      return;
    }

    if (path === "/v1/jobs/job-e2e-dead/replay") {
      await route.fulfill({
        status: 202,
        json: {
          data: {
            id: "job-e2e-dead",
            type: "claim_finalization",
            entityId: "claim-e2e-dashboard",
            status: "pending",
            attempts: 0,
            runAfter: "2026-07-09T15:02:00Z",
            deadLetter: false,
            createdAt: "2026-07-09T15:00:00Z",
            updatedAt: "2026-07-09T15:02:00Z"
          }
        }
      });
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
