import { expect, test } from "@orieken/saturday-playwright";
import type { Page } from "@playwright/test";

import { dashboardUrl } from "./config.js";

const transactionTypes = ["834", "820", "270", "271", "275", "278", "837", "837D", "835", "824", "TA1", "276", "277", "269", "999", "277CA"] as const;

type DemoTransactionType = (typeof transactionTypes)[number];

type DemoTransaction = {
  id: string;
  type: DemoTransactionType;
  status: string;
  senderId: string;
  receiverId: string;
  payload: Record<string, unknown>;
  rawX12: string;
  relatedId?: string;
  createdAt: string;
};

type DemoInboundMessage = {
  id: string;
  partnerId?: string;
  contentType: string;
  transactionType?: string;
  rawPayload: string;
  status: string;
  error?: string;
  downstreamStatus?: number;
  createdAt: string;
};

const demoTransactions: DemoTransaction[] = transactionTypes.map((type, index) => ({
  id: `tx-e2e-${type.toLowerCase()}`,
  type,
  status: type === "999" || type === "275" ? "Accepted" : "Dispatched",
  senderId: type === "834" || type === "820" || type === "824" || type === "TA1" || type === "999" || type === "277CA" ? "Adventure Society" : "provider-vitesse-temple",
  receiverId: type === "834" || type === "820" || type === "824" || type === "TA1" || type === "999" || type === "277CA" ? "provider-vitesse-temple" : "Adventure Society",
  payload: {
    x12: `${type} dashboard display fixture`,
    claimId: `claim-e2e-${type.toLowerCase()}`,
    adventurerId: "adv-e2e-dashboard",
    ...(type === "277"
      ? {
          claimId: "claim-e2e-dashboard",
          adjudication: {
            engine: "async-worker",
            allowedAmountCents: 10000,
            paidAmountCents: 8800,
            patientResponsibilityCents: 1200,
            adjustmentAmountCents: 2500,
            adjustmentReason: "ASHN contractual allowance with current premium",
            coverageStatus: "Active",
            providerTier: "Diamond",
            adventurerRank: "Gold",
            premiumCurrent: true,
            premiumPaidAmountCents: 5000
          }
        }
      : {}),
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
  relatedId: type === "277" ? "tx-e2e-837" : (index > 0 ? "tx-e2e-834" : undefined),
  createdAt: new Date(Date.UTC(2026, 6, 8, 12, index, 0)).toISOString()
}));

const demoInboundMessages: DemoInboundMessage[] = [
  {
    id: "msg-e2e-rejected-837",
    partnerId: "tp-vitesse-temple",
    contentType: "application/xml",
    transactionType: "837",
    rawPayload: "<AshnX12Transaction type=\"837\"><Diagnosis><Code>M542</Code></Diagnosis></AshnX12Transaction>",
    status: "rejected",
    error: "diagnosis code M542 is not allowed for trading partner tp-vitesse-temple; allowed: S610, T509, S062X9A",
    downstreamStatus: 400,
    createdAt: new Date(Date.UTC(2026, 6, 8, 13, 0, 0)).toISOString()
  },
  {
    id: "msg-e2e-accepted-834",
    partnerId: "tp-greenstone-guild",
    contentType: "application/xml",
    transactionType: "834",
    rawPayload: "<AshnX12Transaction type=\"834\" />",
    status: "accepted",
    downstreamStatus: 201,
    createdAt: new Date(Date.UTC(2026, 6, 8, 13, 1, 0)).toISOString()
  },
  {
    id: "msg-e2e-rejected-837-older",
    partnerId: "tp-vitesse-temple",
    contentType: "application/xml",
    transactionType: "837",
    rawPayload: "<AshnX12Transaction type=\"837\"><Diagnosis><Code>BAD</Code></Diagnosis></AshnX12Transaction>",
    status: "rejected",
    error: "diagnosis code BAD is not allowed for trading partner tp-vitesse-temple",
    downstreamStatus: 400,
    createdAt: new Date(Date.UTC(2026, 6, 7, 13, 0, 0)).toISOString()
  }
];

test.describe("ASHN dashboard smoke", () => {
  test.skip(!dashboardUrl, "Set ASHN_DASHBOARD_URL to run dashboard browser smoke tests.");

  test("renders the RPG dashboard shell and service links", async ({ consoleLogger, page }) => {
    await mockDashboardApi(page);
    await page.goto(dashboardUrl);

    await expect(page.getByRole("heading", { name: /ASHN Transaction Dashboard/i })).toBeVisible();
    await expect(page.getByText("Adventure Society Health Network")).toBeVisible();
    await expect(page.locator(".gateway-url")).toHaveAttribute("href", /^https?:\/\/.+/);
    await expect(page.getByLabel("System readiness")).toContainText("ready");
    await expect(page.getByText("5/5 checks ok")).toBeVisible();

    await expect(page.getByRole("button", { name: /Workflow/i })).toBeVisible();
    await expect(page.getByRole("button", { name: /Metrics/i })).toBeVisible();
    await expect(page.getByRole("button", { name: /Timeline/i })).toBeVisible();
    await expect(page.getByRole("button", { name: /Ledger/i })).toBeVisible();
    await expect(page.getByRole("button", { name: /XML Intake/i })).toBeVisible();
    await expect(page.getByRole("button", { name: /Partners/i })).toBeVisible();

    await page.getByRole("button", { name: /Ledger/i }).click();
    await expect(page.getByLabel("Transaction type")).toContainText("275");
    await page.getByLabel("Transaction type").selectOption("275");
    await expect(page.getByLabel("Transaction type")).toHaveValue("275");

    await page.getByRole("button", { name: /Metrics/i }).click();
    await expect(page.getByRole("heading", { name: "Metrics Cockpit" })).toBeVisible();
    await expect(page.getByLabel("Guild Operations Metrics")).toContainText("Loaded Transactions");
    await expect(page.getByText("Transactions by Type")).toBeVisible();
    await expect(page.getByRole("heading", { name: "Claim Money Flow" })).toBeVisible();

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

  test("separates acknowledgment and business review drilldowns", async ({ page }) => {
    await mockDashboardApi(page);
    await page.goto(dashboardUrl);

    await page.getByRole("button", { name: /Ledger/i }).click();
    const drilldown = page.getByLabel("acknowledgment outcome drilldown");

    await expect(drilldown.getByText("TA1 Interchange")).toBeVisible();
    await expect(drilldown.getByText("999 Syntax")).toBeVisible();
    await expect(drilldown.getByText("824 Application")).toBeVisible();
    await expect(drilldown.getByText("Business Review")).toBeVisible();
    await expect(drilldown.getByText("Envelope or interchange pre-screen failures before transaction translation.")).toBeVisible();
    await expect(drilldown.getByText("Manual authorization or attachment review outcomes after EDI acceptance.")).toBeVisible();

    await drilldown.getByRole("button", { name: "Drill into TA1" }).click();
    await expect(page.getByLabel("Transaction type")).toHaveValue("TA1");
    await expect(page.locator(".compact-row").filter({ hasText: "tx-e2e-ta1" })).toBeVisible();
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

  test("records an 820 premium payment from the workflow", async ({ page }) => {
    await mockDashboardApi(page);
    await page.goto(dashboardUrl);

    await page.getByRole("button", { name: "Send 834 Enrollment" }).click();
    await expect(page.locator(".result").getByText("Filter Fixture Ranger", { exact: true })).toBeVisible();

    const premiumResponse = page.waitForResponse((response) => response.url().includes("/v1/premium-payments"));
    await page.getByRole("button", { name: "820 Pay Premium" }).click();
    await premiumResponse;

    const latestEvent = page.locator(".event").first();
    await expect(latestEvent.locator("p").filter({ hasText: "Guild dues payment recorded." })).toBeVisible();
    await latestEvent.getByText("Raw payload").click();
    await expect(latestEvent.getByText("tx-e2e-820-premium")).toBeVisible();

    await page.getByRole("button", { name: /Ledger/i }).click();
    const premiumLedger = page.locator(".panel", { hasText: "Premium Ledger" });
    await expect(premiumLedger.getByText("$50.00 · Reconciled")).toBeVisible();
    await expect(premiumLedger.getByText("Benefit-current · Accepted")).toBeVisible();
    await expect(premiumLedger.getByText("tx-e2e-820-premium")).toBeVisible();

    await premiumLedger.getByText("tx-e2e-820-premium").click();
    const drawer = page.getByLabel("Selected record details");
    await expect(drawer.getByRole("heading", { name: "Premium Reconciliation" })).toBeVisible();
    await expect(drawer.getByText("Premium Payment ID")).toBeVisible();
    await expect(drawer.getByText("premium-e2e-820")).toBeVisible();
    await expect(drawer.getByText("Benefit Current")).toBeVisible();
    await expect(drawer.locator(".detail-item").filter({ hasText: "Benefit Current" }).getByText("Yes")).toBeVisible();

    const downloadPromise = page.waitForEvent("download");
    await drawer.getByRole("button", { name: "Export CSV" }).click();
    const download = await downloadPromise;
    expect(download.suggestedFilename()).toBe("ashn-premium-payment-premium-e2e-820.csv");
  });

  test("submits raw X12 intake from the XML tab", async ({ page }) => {
    await mockDashboardApi(page);
    await page.goto(dashboardUrl);

    await page.getByRole("button", { name: /XML Intake/i }).click();
    await expect(page.getByRole("heading", { name: /Raw X12 Intake/i })).toBeVisible();
    await expect(page.getByLabel("Raw X12")).toContainText("ST*837");
    await page.getByRole("button", { name: "Load Sample 834" }).click();
    await expect(page.getByLabel("Raw X12")).toContainText("ST*834");
    const raw834Response = page.waitForResponse((response) => response.url().includes("/v1/x12/raw"));
    await page.getByRole("button", { name: "Submit Raw X12" }).click();
    await raw834Response;
    await page.getByRole("button", { name: /Workflow/i }).click();
    const latest834Event = page.locator(".event").first();
    await expect(latest834Event.locator("p").filter({ hasText: "Raw X12 enrollment accepted." })).toBeVisible();
    await latest834Event.getByText("Raw payload").click();
    await expect(latest834Event.getByText("tx-e2e-raw-834")).toBeVisible();

    await page.getByRole("button", { name: /XML Intake/i }).click();
    await page.getByRole("button", { name: "Load Sample 820" }).click();
    await expect(page.getByLabel("Raw X12")).toContainText("ST*820");
    const raw820Response = page.waitForResponse((response) => response.url().includes("/v1/x12/raw"));
    await page.getByRole("button", { name: "Submit Raw X12" }).click();
    await raw820Response;
    await page.getByRole("button", { name: /Workflow/i }).click();
    const latest820Event = page.locator(".event").first();
    await expect(latest820Event.locator("p").filter({ hasText: "Raw X12 premium accepted." })).toBeVisible();
    await latest820Event.getByText("Raw payload").click();
    await expect(latest820Event.getByText("tx-e2e-raw-820")).toBeVisible();

    await page.getByRole("button", { name: /XML Intake/i }).click();
    await page.getByRole("button", { name: "Load Sample 270" }).click();
    await expect(page.getByLabel("Raw X12")).toContainText("ST*270");

    const rawResponse = page.waitForResponse((response) => response.url().includes("/v1/x12/raw"));
    await page.getByRole("button", { name: "Submit Raw X12" }).click();
    await rawResponse;
    await page.getByRole("button", { name: /Workflow/i }).click();
    const latestEvent = page.locator(".event").first();
    await expect(latestEvent.locator("p").filter({ hasText: "Raw X12 eligibility checked." })).toBeVisible();
    await latestEvent.getByText("Raw payload").click();
    await expect(latestEvent.getByText("tx-e2e-raw-270")).toBeVisible();

    await page.getByRole("button", { name: /XML Intake/i }).click();
    await page.getByRole("button", { name: "Load Sample 276" }).click();
    await expect(page.getByLabel("Raw X12")).toContainText("ST*276");
    const raw276Response = page.waitForResponse((response) => response.url().includes("/v1/x12/raw"));
    await page.getByRole("button", { name: "Submit Raw X12" }).click();
    await raw276Response;
    await page.getByRole("button", { name: /Workflow/i }).click();
    const latest276Event = page.locator(".event").first();
    await expect(latest276Event.locator("p").filter({ hasText: "Raw X12 claim status checked." })).toBeVisible();
    await latest276Event.getByText("Raw payload").click();
    await expect(latest276Event.getByText("tx-e2e-raw-276")).toBeVisible();

    await page.getByRole("button", { name: /XML Intake/i }).click();
    await page.getByRole("button", { name: "Load Sample 278" }).click();
    await expect(page.getByLabel("Raw X12")).toContainText("ST*278");
    const raw278Response = page.waitForResponse((response) => response.url().includes("/v1/x12/raw"));
    await page.getByRole("button", { name: "Submit Raw X12" }).click();
    await raw278Response;
    await page.getByRole("button", { name: /Workflow/i }).click();
    const latest278Event = page.locator(".event").first();
    await expect(latest278Event.locator("p").filter({ hasText: "Raw X12 prior authorization queued." })).toBeVisible();
    await latest278Event.getByText("Raw payload").click();
    await expect(latest278Event.getByText("tx-e2e-raw-278")).toBeVisible();

    await page.getByRole("button", { name: /XML Intake/i }).click();
    await page.getByRole("button", { name: "Load Sample 835" }).click();
    await expect(page.getByLabel("Raw X12")).toContainText("ST*835");
    const raw835Response = page.waitForResponse((response) => response.url().includes("/v1/x12/raw"));
    await page.getByRole("button", { name: "Submit Raw X12" }).click();
    await raw835Response;
    await page.getByRole("button", { name: /Workflow/i }).click();
    const latest835Event = page.locator(".event").first();
    await expect(latest835Event.locator("p").filter({ hasText: "Raw X12 payment accepted." })).toBeVisible();
    await latest835Event.getByText("Raw payload").click();
    await expect(latest835Event.getByText("tx-e2e-raw-835")).toBeVisible();
  });

  test("shows operational audit dashboard for partner rejections", async ({ page }) => {
    await mockDashboardApi(page);
    await page.goto(dashboardUrl);

    await page.getByRole("button", { name: /XML Intake/i }).click();
    await expect(page.getByRole("heading", { name: /Partner Rejection Ops/i })).toBeVisible();
    await expect(page.getByText("2 rejected")).toBeVisible();
    await expect(page.getByLabel("rejection trend")).toBeVisible();
    await expect(page.getByText("tp-vitesse-temple · 2")).toBeVisible();
    await expect(page.getByText("837 · 2")).toBeVisible();
    await expect(page.getByText("Diagnosis code profile · 2")).toBeVisible();
    await expect(page.getByLabel("intake rejections").getByText("diagnosis code M542 is not allowed")).toBeVisible();

    await page.getByLabel("intake rejections").getByRole("button", { name: "Diagnosis code profile · 2" }).click();
    await expect(page.getByPlaceholder("Search IDs, names, statuses, providers...")).toHaveValue("diagnosis code");
    await expect(page.getByLabel("XML status")).toHaveValue("rejected");

    await page.getByRole("button", { name: "Rejected 837s" }).click();
    await expect(page.getByLabel("XML status")).toHaveValue("rejected");
    await expect(page.getByLabel("XML type")).toHaveValue("837");

    const m542Rejection = page.locator(".document-review-row", { hasText: "M542" });
    const replayResponse = page.waitForResponse((response) => response.url().includes("/v1/x12/messages/msg-e2e-rejected-837/replay"));
    await m542Rejection.getByRole("button", { name: "Replay" }).click();
    const response = await replayResponse;
    expect(response.status()).toBe(202);
    await expect(response.json()).resolves.toMatchObject({ lore: "Replay queued for rejected 837 intake." });

    await m542Rejection.getByRole("button", { name: "Inspect" }).click();
    await expect(page.getByRole("heading", { name: /Intake Detail/i })).toBeVisible();
    await expect(page.getByLabel("Selected record details").getByText("msg-e2e-rejected-837")).toBeVisible();
  });

  test("submits a multipart batch file drop", async ({ page }) => {
    await mockDashboardApi(page);
    await page.goto(dashboardUrl);

    await page.getByRole("button", { name: /XML Intake/i }).click();
    await expect(page.getByRole("heading", { name: "Batch File Drop" })).toBeVisible();
    await page.getByLabel("Intake files").setInputFiles([
      {
        name: "eligibility.xml",
        mimeType: "application/xml",
        buffer: Buffer.from("<AshnX12Transaction type=\"270\" />")
      },
      {
        name: "claim.x12",
        mimeType: "application/edi-x12",
        buffer: Buffer.from("ISA*00*")
      }
    ]);
    const batchResponse = page.waitForResponse((response) => response.url().includes("/v1/x12/batch"));
    await page.getByRole("button", { name: "Submit Batch" }).click();
    const response = await batchResponse;
    expect(response.status()).toBe(207);
    await expect(response.json()).resolves.toMatchObject({
      data: {
        total: 2,
        accepted: 1,
        rejected: 1
      }
    });
  });

  test("runs 275 rejection fixture tour", async ({ page }) => {
    await mockDashboardApi(page);
    await page.goto(dashboardUrl);

    await page.getByRole("button", { name: /XML Intake/i }).click();
    await page.getByRole("button", { name: "275 Invalid BGN01" }).click();
    await expect(page.getByLabel("Raw X12")).toContainText("BGN*99");
    await page.getByRole("button", { name: "275 Invalid CAT02" }).click();
    await expect(page.getByLabel("Raw X12")).toContainText("CAT*B4*BIN");

    await page.getByRole("button", { name: /Workflow/i }).click();
    const scenario = page.locator(".scenario-card", { hasText: "275 Rejection Fixture Tour" });
    await expect(scenario).toBeVisible();
    await scenario.getByRole("button", { name: "Run Scenario" }).click();
    await expect(page.getByLabel("XML type")).toHaveValue("275");
    await expect(page.getByLabel("XML status")).toHaveValue("rejected");
    const rejections = page.getByLabel("intake rejections");
    await expect(rejections.getByRole("button", { name: /Attachment purpose profile/ })).toBeVisible();
    await expect(rejections.getByRole("button", { name: /Attachment format profile/ })).toBeVisible();
    await expect(rejections.getByRole("button", { name: /Attachment payload encoding/ })).toBeVisible();
    await expect(rejections.getByRole("button", { name: /Solicited trace matching/ })).toBeVisible();
    await expect(rejections.getByRole("button", { name: /Late unsolicited attachment/ })).toBeVisible();
  });

  test("exports the loaded transaction ledger to CSV", async ({ page }) => {
    await mockDashboardApi(page);
    await page.goto(dashboardUrl);

    await page.getByRole("button", { name: /Ledger/i }).click();
    await page.getByLabel("Transaction type").selectOption("275");
    await expect(page.getByText("tx-e2e-275")).toBeVisible();

    const downloadPromise = page.waitForEvent("download");
    await page.getByRole("button", { name: "Export CSV" }).click();
    const download = await downloadPromise;

    expect(download.suggestedFilename()).toBe("ashn-ledger-transactions.csv");
  });

  test("exports repeatable demo scenario runbooks", async ({ page }) => {
    await mockDashboardApi(page);
    await page.goto(dashboardUrl);

    await expect(page.getByRole("heading", { name: /Exportable Demo Scenarios/i })).toBeVisible();
    await expect(page.getByRole("heading", { name: /Premium-Current Claim Adjudication/i })).toBeVisible();

    const downloadPromise = page.waitForEvent("download");
    await page.locator(".scenario-card").filter({ hasText: "Premium-Current Claim Adjudication" }).getByRole("button", { name: "Export Scenario JSON" }).click();
    const download = await downloadPromise;

    expect(download.suggestedFilename()).toBe("ashn-demo-scenario-premium-current-claim.json");
  });

  test("runs a premium-current demo scenario", async ({ page }) => {
    await mockDashboardApi(page);
    await page.addInitScript(() => {
      if (!window.sessionStorage.getItem("ashn.e2e.clearedScenarioRuns")) {
        window.localStorage.removeItem("ashn.scenarioRuns.v1");
        window.sessionStorage.setItem("ashn.e2e.clearedScenarioRuns", "true");
      }
    });
    await page.goto(dashboardUrl);

    const scenario = page.locator(".scenario-card").filter({ hasText: "Premium-Current Claim Adjudication" });
    await scenario.getByRole("button", { name: "Run Scenario" }).click();

    await expect(scenario.getByText("Complete")).toBeVisible();
    await expect(scenario.getByText("4/4 steps")).toBeVisible();
    await expect(page.locator(".event p").filter({ hasText: "Scenario claim submitted." })).toBeVisible();

    const recentRuns = page.getByLabel("Recent scenario runs");
    await expect(recentRuns.getByText("Premium-Current Claim Adjudication")).toBeVisible();
    await expect(recentRuns.getByText(/4\/4 steps · completed/i)).toBeVisible();
    await expect(recentRuns.getByText("4 tx")).toBeVisible();

    const recentBundleDownload = page.waitForEvent("download");
    await recentRuns.getByRole("button", { name: "Export Evidence" }).first().click();
    const recentDownload = await recentBundleDownload;
    expect(recentDownload.suggestedFilename()).toMatch(/^ashn-demo-evidence-premium-current-claim-scenario-premium-current-claim-\d+\.json$/);

    const bundleDownload = page.waitForEvent("download");
    await scenario.getByRole("button", { name: "Export Evidence Bundle" }).click();
    const download = await bundleDownload;
    expect(download.suggestedFilename()).toBe("ashn-demo-evidence-premium-current-claim.json");

    await page.reload();
    const reloadedRuns = page.getByLabel("Recent scenario runs");
    await expect(reloadedRuns.getByText("Premium-Current Claim Adjudication")).toBeVisible();
    await expect(reloadedRuns.getByText("4 tx")).toBeVisible();
  });

  test("runs the dental predetermination to remittance scenario", async ({ page }) => {
    await mockDashboardApi(page);
    await page.addInitScript(() => {
      window.localStorage.removeItem("ashn.scenarioRuns.v1");
    });
    await page.goto(dashboardUrl);

    const scenario = page.locator(".scenario-card").filter({ hasText: "Dental Predetermination to Remittance" });
    await expect(scenario).toBeVisible();
    await expect(scenario.getByText("270 dental eligibility")).toBeVisible();
    await expect(scenario.getByText("835 CDT remittance")).toBeVisible();

    await scenario.getByRole("button", { name: "Run Scenario" }).click();

    await expect(scenario.getByText("Complete")).toBeVisible();
    await expect(scenario.getByText("6/6 steps")).toBeVisible();
    await expect(page.locator(".event p").filter({ hasText: "Dental claim paid." })).toBeVisible();
    await page.getByRole("button", { name: "Close" }).click();
    const remittanceEvent = page.locator(".event").filter({ hasText: "835" }).first();
    await remittanceEvent.getByText("Raw payload").click();
    await expect(remittanceEvent.getByText("D7240")).toBeVisible();
    await expect(page.getByLabel("Recent scenario runs").getByText("Dental Predetermination to Remittance")).toBeVisible();
  });

  test("plays a demo scenario one step at a time", async ({ page }) => {
    await mockDashboardApi(page);
    await page.addInitScript(() => {
      if (!window.sessionStorage.getItem("ashn.e2e.clearedScenarioRuns")) {
        window.localStorage.removeItem("ashn.scenarioRuns.v1");
        window.sessionStorage.setItem("ashn.e2e.clearedScenarioRuns", "true");
      }
    });
    await page.goto(dashboardUrl);

    const scenario = page.locator(".scenario-card").filter({ hasText: "Premium-Current Claim Adjudication" });
    await scenario.getByRole("button", { name: "Start Playback" }).click();
    await expect(scenario.getByText("Playback ready: Enroll")).toBeVisible();
    await expect(scenario.getByText("0/4 steps")).toBeVisible();

    await scenario.getByRole("button", { name: "Run Next Step" }).click();
    await expect(scenario.getByText("Playback ready: Premium")).toBeVisible();
    await expect(scenario.getByText("1/4 steps")).toBeVisible();

    await scenario.getByRole("button", { name: "Run Next Step" }).click();
    await expect(scenario.getByText("Playback ready: Claim")).toBeVisible();
    await expect(scenario.getByText("2/4 steps")).toBeVisible();
    await expect(page.locator(".event p").filter({ hasText: "Guild dues payment recorded." })).toBeVisible();

    await scenario.getByRole("button", { name: "Finish Playback" }).click();
    await expect(scenario.getByText("Complete")).toBeVisible();
    await expect(scenario.getByText("4/4 steps")).toBeVisible();
    await expect(page.getByLabel("Recent scenario runs").getByText("Premium-Current Claim Adjudication")).toBeVisible();
  });

  test("labels 275 claim attachments inside the transaction timeline", async ({ page }) => {
    await mockDashboardApi(page);
    await page.goto(dashboardUrl);

    await page.getByRole("button", { name: /Timeline/i }).click();
    await page.getByLabel("Transaction type").selectOption("275");

    await expect(page.getByText("Claim lifecycle").first()).toBeVisible();
    await expect(page.getByText("Claim claim-e2e-275")).toBeVisible();
    await expect(page.getByRole("button", { name: /275 OZ\/B4 attachment/i })).toBeVisible();
    await expect(page.getByRole("button", { name: /packet-e2e-275 \(1\/2\)/i })).toBeVisible();
    await expect(page.getByLabel("Attachment packet summary").getByText("275 Packet Summary")).toBeVisible();
    await expect(page.getByLabel("Attachment packet summary").getByRole("button", { name: /packet-e2e-275.*1\/2 docs observed.*Received/i })).toBeVisible();
  });

  test("shows visual request response links in transaction detail", async ({ page }) => {
    await mockDashboardApi(page);
    await page.goto(dashboardUrl);

    await page.getByRole("button", { name: /Ledger/i }).click();
    await page.getByText("tx-e2e-834").click();

    const drawer = page.getByLabel("Selected record details");
    await expect(drawer.getByRole("heading", { name: /Transaction Detail/i })).toBeVisible();
    await expect(drawer.getByRole("heading", { name: /Request \/ Response Links/i })).toBeVisible();
    await expect(drawer.getByText("No parent")).toBeVisible();
    await expect(drawer.getByRole("button", { name: /Current 834 · Dispatched tx-e2e-834/i })).toBeVisible();
    await expect(drawer.getByRole("button", { name: /Response 275 · Accepted tx-e2e-275/i })).toBeVisible();

    await drawer.getByRole("button", { name: /Response 275 · Accepted tx-e2e-275/i }).click();
    await expect(drawer.locator(".detail-item").filter({ hasText: "Related" }).getByText("tx-e2e-834")).toBeVisible();
    await expect(drawer.getByRole("button", { name: /Source 834 · Dispatched tx-e2e-834/i })).toBeVisible();
  });

  test("manages trading partner profiles", async ({ page }) => {
    await mockDashboardApi(page);
    await page.goto(dashboardUrl);

    await page.getByRole("button", { name: /Partners/i }).click();
    const vitesseCard = page.locator(".partner-card").filter({ hasText: "Temple of the Healer, Vitesse" });
    await expect(vitesseCard.getByLabel("Temple of the Healer, Vitesse companion guide")).toBeVisible();
    await expect(vitesseCard.getByText("275 Attachments")).toBeVisible();
    await expect(vitesseCard.getByText("278 Auth")).toBeVisible();
    await expect(vitesseCard.getByText(/resurrection\/restoration\/curse-removal.*services/)).toBeVisible();
    await expect(vitesseCard.getByText("837 Claims")).toBeVisible();
    await expect(vitesseCard.getByText(/S610\/T509.*diagnoses/)).toBeVisible();
    await expect(vitesseCard.getByText("Dental Rules")).toBeVisible();
    await expect(vitesseCard.getByText(/D7000-D7999 CDT/)).toBeVisible();
    await expect(vitesseCard.getByText(/tooth required/)).toBeVisible();
    await expect(vitesseCard.getByText(/XRAY\/PERIO\/NARR\/PLAN docs/)).toBeVisible();

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

  test("queues a dental 278 predetermination with tooth details", async ({ page }) => {
    await mockDashboardApi(page);
    await page.goto(dashboardUrl);

    await page.getByRole("button", { name: /Send 834 Enrollment/i }).click();
    await page.getByRole("button", { name: /278 Dental Predetermination/i }).click();

    const authReview = page.locator(".auth-review-card");
    await expect(authReview.getByText("278 · Pending")).toBeVisible();
    await expect(authReview.getByText("dental predetermination")).toBeVisible();
    await expect(authReview.getByText("CDT D7240", { exact: true })).toBeVisible();
    await expect(authReview.getByText("Tooth 14", { exact: true })).toBeVisible();
    await expect(authReview.getByText("Surface MO", { exact: true })).toBeVisible();

    const latestEvent = page.locator(".event").first();
    await latestEvent.getByText("Raw payload").click();
    await expect(latestEvent.getByText("dental-predetermination")).toBeVisible();
    await expect(latestEvent.getByText("D7240")).toBeVisible();
  });

  test("checks dental eligibility with benefit limits", async ({ page }) => {
    await mockDashboardApi(page);
    await page.goto(dashboardUrl);

    await page.getByRole("button", { name: /Send 834 Enrollment/i }).click();
    await page.getByRole("button", { name: /270 → 271 Dental Eligibility/i }).click();

    const latestEvent = page.locator(".event").first();
    await expect(latestEvent.getByText("270 · Dispatched")).toBeVisible();
    await expect(latestEvent.getByText("271 · Accepted")).toBeVisible();
    await expect(latestEvent.locator("p").filter({ hasText: "Dental eligibility checked." })).toBeVisible();
    await latestEvent.getByText("Raw payload").click();
    await expect(latestEvent.getByText("annualMaximumCents")).toBeVisible();
    await expect(latestEvent.getByText("remainingMaximumCents")).toBeVisible();
    await expect(latestEvent.getByText("2 cleanings per plan year")).toBeVisible();
  });

  test("submits an 837D dental claim with CDT tooth details", async ({ page }) => {
    await mockDashboardApi(page);
    await page.goto(dashboardUrl);

    await page.getByRole("button", { name: /Send 834 Enrollment/i }).click();
    await page.getByRole("button", { name: /837D Submit Dental Claim/i }).click();

    const latestEvent = page.locator(".event").first();
    await expect(latestEvent.getByText("837D · Dispatched")).toBeVisible();
    await expect(latestEvent.locator("p").filter({ hasText: "Dental claim submitted." })).toBeVisible();
    await latestEvent.getByText("Raw payload").click();
    await expect(latestEvent.getByText("837D dashboard display fixture")).toBeVisible();
    await expect(latestEvent.getByText("D7240")).toBeVisible();
    await expect(latestEvent.getByText("toothNumber")).toBeVisible();

    await page.getByRole("button", { name: /Ledger/i }).click();
    await page.getByLabel("Transaction type").selectOption("837D");
    await expect(page.getByText("tx-e2e-837d")).toBeVisible();
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

  test("reviews 275 authorization documentation packet", async ({ page }) => {
    await mockDashboardApi(page);
    await page.goto(dashboardUrl);

    await page.getByRole("button", { name: /Send 834 Enrollment/i }).click();
    await page.getByRole("button", { name: /278 Resurrection Auth/i }).click();

    const authReview = page.locator(".auth-review-card");
    await expect(authReview.getByRole("heading", { name: /278 Authorization Documentation/i })).toBeVisible();
    await expect(authReview.getByText("No 275 support is linked")).toBeVisible();

    await authReview.getByRole("button", { name: /Submit Auth 275 Packet/i }).click();
    await expect(authReview.getByText("2 auth docs received")).toBeVisible();
    await expect(authReview.getByText("Authorization Document Review")).toBeVisible();

    const firstDoc = authReview.locator(".document-review-row").filter({ hasText: "Medical necessity letter for authorization" });
    await expect(firstDoc.getByText("Received")).toBeVisible();
    await firstDoc.getByRole("button", { name: "Accept Doc" }).click();
    await expect(firstDoc.getByText("Accepted")).toBeVisible();
  });

  test("requests 275 documentation from claim detail", async ({ page }) => {
    await mockDashboardApi(page);
    await page.goto(dashboardUrl);

    await page.getByRole("button", { name: /Ledger/i }).click();
    await page.getByText("claim-e2e-dashboard").click();
    const drawer = page.getByLabel("Selected record details");
    await expect(drawer.getByRole("heading", { name: /Claim Detail/i })).toBeVisible();
    await expect(drawer.locator(".detail-item").filter({ hasText: "Status" }).getByText("Submitted", { exact: true })).toBeVisible();
    await expect(drawer.locator(".detail-item").filter({ hasText: "Prior Auth" }).getByText("tx-e2e-auth-review")).toBeVisible();
    await expect(drawer.locator(".detail-item").filter({ hasText: "Auth Status" }).getByText("Approved")).toBeVisible();
    await expect(drawer.getByRole("heading", { name: /Adjudication Explanation/i })).toBeVisible();
    await expect(drawer.getByText("Premium Current: Yes")).toBeVisible();
    await expect(drawer.getByText("Premium Paid: $50.00")).toBeVisible();
    await expect(drawer.getByText("ASHN contractual allowance with current premium")).toBeVisible();
    await expect(drawer.getByRole("heading", { name: /275 Documentation Workbench/i })).toBeVisible();
    await expect(drawer.getByText("Medical necessity letter")).toBeVisible();
    await expect(drawer.getByText("Encounter notes")).toBeVisible();

    await page.getByRole("button", { name: /Request 275 Docs/i }).click();
    await expect(drawer.getByText("Pending Documentation")).toBeVisible();

    await page.getByRole("button", { name: /Submit 275 Packet/i }).click();
    await expect(drawer.getByText("Pending", { exact: true })).toBeVisible();
    await expect(drawer.getByText("Document Review")).toBeVisible();
    await expect(drawer.locator(".document-review-row").filter({ hasText: "Medical necessity letter" })).toBeVisible();
    await drawer.locator(".document-review-row").filter({ hasText: "Medical necessity letter" }).getByRole("button", { name: "Accept Doc" }).click();
    await expect(drawer.locator(".document-review-row").filter({ hasText: "Medical necessity letter" }).getByText("Accepted")).toBeVisible();
    await expect(drawer.locator(".document-review-row").filter({ hasText: "Encounter notes" }).getByText("Received")).toBeVisible();
    const encounterRow = drawer.locator(".document-review-row").filter({ hasText: "Encounter notes" });
    await encounterRow.getByRole("button", { name: "Reject Doc" }).click();
    await expect(encounterRow.getByText("Rejected")).toBeVisible();
    await encounterRow.getByRole("button", { name: "Request + Resubmit" }).click();
    await expect(drawer.locator(".document-review-row").filter({ hasText: "Encounter notes resubmission" }).getByText("Received")).toBeVisible();
    await expect(drawer.locator(".document-review-row").filter({ hasText: "Medical necessity letter" }).getByText("Accepted")).toBeVisible();

    await page.getByRole("button", { name: "Close" }).click();
    await page.getByRole("button", { name: /Workflow/i }).click();
    const latestEvent = page.locator(".event").first();
    await latestEvent.getByText("Raw payload").click();
    await expect(latestEvent.getByText("tx-e2e-doc-packet-4")).toBeVisible();
    await expect(latestEvent.getByText("Encounter notes resubmission")).toBeVisible();
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

    const receiptResponse = page.waitForResponse((response) => response.url().includes("/v1/transactions/tx-e2e-275/document-reference"));
    await drawer.getByRole("button", { name: /Inspect Vault Receipt/i }).click();
    await receiptResponse;
    await drawer.getByRole("button", { name: "Close" }).click();
    await page.getByRole("button", { name: /Workflow/i }).click();
    const latestEvent = page.locator(".event").first();
    await expect(latestEvent.locator("p").filter({ hasText: "document vault resolved" })).toBeVisible();
    await latestEvent.getByText("Raw payload").click();
    await expect(latestEvent.getByText("external-reference")).toBeVisible();
    await expect(latestEvent.getByText("doc-e2e-275")).toBeVisible();

    await page.getByRole("button", { name: /Ledger/i }).click();
    await page.getByLabel("Transaction type").selectOption("275");
    await page.getByText("tx-e2e-275").click();
    await drawer.getByRole("button", { name: /Reject Attachment/i }).click();
    await expect(reviewRow.getByText("Rejected", { exact: true })).toBeVisible();
    await expect(drawer.locator(".detail-item").filter({ hasText: "Review Reason" }).getByText("Supporting documentation is insufficient for business review.")).toBeVisible();
    await expect(drawer.locator(".detail-item").filter({ hasText: "Status" }).getByText("Accepted", { exact: true })).toBeVisible();
  });
});

async function mockDashboardApi(page: Page) {
  let claimStatus = "Submitted";
  const workbenchTransactions: DemoTransaction[] = [];
  const rejected275Messages: DemoInboundMessage[] = [];
  const premiumPayments: Array<{
    id: string;
    adventurerId: string;
    transactionId: string;
    amountCents: number;
    status: string;
    createdAt: string;
    reconciled: boolean;
    currentForBenefits: boolean;
  }> = [];
  const partnerProfiles = [
    {
      id: "tp-vitesse-temple",
      name: "Temple of the Healer, Vitesse",
      senderId: "provider-vitesse-temple",
      receiverId: "Adventure Society",
      allowedTransactionTypes: [...transactionTypes],
      routeTarget: "payer-core",
      status: "active",
      validationProfile: {
        attachmentTypes: ["OZ"],
        reportTypeCodes: ["B4"],
        contentTypes: ["text/plain"],
        allowedFileExtensions: [".txt"],
        maxAttachmentsPerPacket: 3,
        unsolicitedAttachmentWindowDays: 0,
        maxEmbeddedContentBytes: 2048,
        serviceTypes: ["resurrection", "restoration", "curse-removal", "dental-predetermination"],
        incidentSeverities: ["Normal", "Awakened"],
        diagnosisCodes: ["S610", "T509", "K021"],
        procedureCodePrefixes: ["ASHN", "D"],
        dentalCdtRanges: ["D7000-D7999"],
        dentalRequiredAttachmentCodes: ["XRAY", "PERIO", "NARR", "PLAN"],
        dentalRequiresToothNumber: true,
        dentalAllowedSurfaces: ["O", "M", "D", "B", "L", "MO", "DO", "MOD"],
        dentalAllowedQuadrants: ["UR", "UL", "LR", "LL"],
        dentalPredeterminationRules: ["oral-surgery-only", "accepted-275-evidence-required"]
      }
    }
  ];
  await page.route("**/v1/**", async (route) => {
    const url = new URL(route.request().url());
    const path = url.pathname;

    if (path === "/v1/health") {
      await route.fulfill({ json: { data: { "api-gateway": "ok", "payer-core": "ok", "provider-service": "ok", "edi-intake": "ok" } } });
      return;
    }

    if (path === "/v1/system/readiness") {
      await route.fulfill({
        json: {
          data: {
            status: "ready",
            generatedAt: new Date(Date.UTC(2026, 6, 19, 4, 38, 0)).toISOString(),
            version: "0.1.0",
            commit: "e2e-readiness",
            services: { "api-gateway": "ok", "payer-core": "ok", "provider-service": "ok", "edi-intake": "ok" },
            checks: [
              { name: "ledger transactions", status: "ok", detail: "reachable", count: 25 },
              { name: "async jobs", status: "ok", detail: "reachable", count: 2 },
              { name: "provider registry", status: "ok", detail: "reachable", count: 1 },
              { name: "intake audit", status: "ok", detail: "reachable", count: 10 },
              { name: "intake rejections", status: "ok", detail: "reachable", count: 4 }
            ],
            summary: { ok: 5, degraded: 0, unavailable: 0 },
            links: { openapi: "/openapi.json", health: "/v1/health" }
          }
        }
      });
      return;
    }

    if (path === "/v1/metrics/summary") {
      await route.fulfill({
        json: {
          data: {
            generatedAt: new Date(Date.UTC(2026, 6, 19, 4, 44, 0)).toISOString(),
            window: "latest 100 records per source",
            transactions: {
              totalLoaded: 17,
              byType: { "837": 4, "275": 5, "835": 2, "278": 3, "999": 3 },
              byStatus: { Accepted: 9, Pending: 3, Paid: 2, Denied: 1, Failed: 2 }
            },
            claims: {
              totalLoaded: 6,
              byStatus: { Paid: 2, Approved: 2, Denied: 1, "Pending Documentation": 1 },
              byProvider: { "provider-vitesse-temple": 4, "provider-greenstone-roadside": 2 }
            },
            intake: {
              rejectionTotal: 7,
              byPartner: { "tp-vitesse-temple": 5, "tp-rimaros-hospital": 2 },
              byType: { "275": 4, "837": 3 },
              byReason: { "diagnosis not allowed": 3, "invalid attachment format": 2, "missing trace": 2 }
            },
            asyncJobs: {
              totalLoaded: 4,
              byStatus: { completed: 2, pending: 1, failed: 1 },
              deadLetters: 1
            },
            financials: {
              billedCents: 250000,
              allowedCents: 190000,
              paidCents: 160000,
              patientResponsibilityCents: 30000,
              adjustmentCents: 60000
            },
            operationalStatus: "attention",
            highlights: ["Partner rejection activity detected", "Paid claims are flowing through remittance"]
          }
        }
      });
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

    if (path === "/v1/premium-payments" && route.request().method() === "POST") {
      premiumPayments.unshift({
        id: "premium-e2e-820",
        adventurerId: "adv-e2e-dashboard",
        transactionId: "tx-e2e-820-premium",
        amountCents: 5000,
        status: "Accepted",
        createdAt: new Date(Date.UTC(2026, 6, 8, 15, 0, 0)).toISOString(),
        reconciled: true,
        currentForBenefits: true
      });
      await route.fulfill({
        status: 201,
        json: {
          data: { adventurerId: "adv-e2e-dashboard", amountCents: 5000, status: "Accepted" },
          lore: "Guild dues payment recorded.",
          transaction: {
            ...demoTransactions.find((transaction) => transaction.type === "820"),
            id: "tx-e2e-820-premium",
            payload: { x12: "820 premium workflow fixture", adventurerId: "adv-e2e-dashboard", amountCents: "5000" },
            rawX12: "ST*820*workflow-premium~BPR*C*50.00~SE*3*workflow-premium~"
          }
        }
      });
      return;
    }

    if (path.startsWith("/v1/premium-payments/") && path.endsWith("/export")) {
      const id = path.split("/").at(-2) ?? "";
      const payment = premiumPayments.find((item) => item.id === id);
      await route.fulfill({
        status: payment ? 200 : 404,
        headers: {
          "content-type": url.searchParams.get("format") === "xml" ? "application/xml" : "application/json",
          "content-disposition": `attachment; filename="ashn-premium-payment-${id}.${url.searchParams.get("format") === "xml" ? "xml" : "json"}"`
        },
        body: url.searchParams.get("format") === "xml"
          ? `<PremiumPayment id="${id}"></PremiumPayment>`
          : JSON.stringify(payment ?? { error: "missing" })
      });
      return;
    }

    if (path.startsWith("/v1/premium-payments/")) {
      const id = path.split("/").pop() ?? "";
      const payment = premiumPayments.find((item) => item.id === id);
      await route.fulfill({
        status: payment ? 200 : 404,
        json: payment ? { data: payment, lore: "The dues ledger opened this 820 reconciliation record." } : { error: "premium payment not found" }
      });
      return;
    }

    if (path === "/v1/premium-payments") {
      await route.fulfill({
        json: {
          data: premiumPayments,
          page: pageInfo(premiumPayments.length, Number(url.searchParams.get("limit") ?? 5))
        }
      });
      return;
    }

    if (path === "/v1/eligibility" && route.request().method() === "POST") {
      const body = route.request().postDataJSON() as { adventurerId: string; providerId: string; serviceType?: string };
      const isDental = body.serviceType === "dental";
      await route.fulfill({
        json: {
          data: {
            eligible: true,
            coverageStatus: "Active",
            adventurerId: body.adventurerId,
            providerId: body.providerId,
            serviceType: body.serviceType,
            ...(isDental
              ? {
                  dentalEligibility: {
                    serviceType: "dental",
                    annualMaximumCents: 150000,
                    remainingMaximumCents: 150000,
                    preventiveCoveragePercent: 100,
                    basicCoveragePercent: 80,
                    majorCoveragePercent: 50,
                    waitingPeriodMonths: 0,
                    frequencyLimit: "2 cleanings per plan year; 1 panoramic image per 36 months"
                  }
                }
              : {})
          },
          lore: isDental ? "Dental eligibility checked." : "Eligibility checked.",
          transactions: [
            {
              ...demoTransactions.find((transaction) => transaction.type === "270"),
              id: isDental ? "tx-e2e-270-dental" : "tx-e2e-270-workflow",
              payload: { x12: isDental ? "270 dental eligibility fixture" : "270 eligibility fixture", adventurerId: body.adventurerId, providerId: body.providerId, serviceType: body.serviceType },
              rawX12: isDental ? "ST*270*dental~EQ*35~SE*3*dental~" : demoTransactions.find((transaction) => transaction.type === "270")!.rawX12
            },
            {
              ...demoTransactions.find((transaction) => transaction.type === "271"),
              id: isDental ? "tx-e2e-271-dental" : "tx-e2e-271-workflow",
              status: "Accepted",
              payload: {
                x12: isDental ? "271 dental benefit response fixture" : "271 eligibility response fixture",
                adventurerId: body.adventurerId,
                eligible: true,
                coverageStatus: "Active",
                serviceType: body.serviceType,
                ...(isDental
                  ? {
                      dentalEligibility: {
                        serviceType: "dental",
                        annualMaximumCents: 150000,
                        remainingMaximumCents: 150000,
                        preventiveCoveragePercent: 100,
                        basicCoveragePercent: 80,
                        majorCoveragePercent: 50,
                        waitingPeriodMonths: 0,
                        frequencyLimit: "2 cleanings per plan year; 1 panoramic image per 36 months"
                      }
                    }
                  : {})
              },
              rawX12: isDental ? "ST*271*dental~EB*1**35~EB*B**35***23*1500.00~EB*C**35***29*1500.00~MSG*Preventive 100% Basic 80% Major 50%~SE*6*dental~" : demoTransactions.find((transaction) => transaction.type === "271")!.rawX12
            }
          ]
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
      const body = route.request().postDataJSON() as {
        adventurerId: string;
        providerId: string;
        serviceType: string;
        incidentSeverity: string;
        dentalService?: {
          cdtCode?: string;
          toothNumber?: string;
          surface?: string;
          quadrant?: string;
          orthodontic?: boolean;
        };
      };
      const isDentalPredetermination = body.serviceType === "dental-predetermination";
      const dentalRequiredDocuments = [
        { code: "XRAY", label: "Diagnostic x-rays", attachmentType: "OZ", reportTypeCode: "B4", contentType: "image/jpeg", required: true },
        { code: "PERIO", label: "Periodontal chart", attachmentType: "OZ", reportTypeCode: "B4", contentType: "text/plain", required: true },
        { code: "NARR", label: "Clinical narrative", attachmentType: "OZ", reportTypeCode: "B4", contentType: "text/plain", required: true },
        { code: "PLAN", label: "Treatment plan", attachmentType: "OZ", reportTypeCode: "B4", contentType: "application/pdf", required: true },
        { code: "ORTHO", label: "Orthodontic records", attachmentType: "OZ", reportTypeCode: "B4", contentType: "application/pdf", required: false }
      ];
      const dentalManualReviewPrompts = [
        "Confirm diagnostic x-rays support the requested CDT procedure.",
        "Review periodontal charting for clinical necessity and supporting tooth/quadrant context.",
        "Read the clinical narrative for symptoms, failed conservative care, and planned outcome.",
        "Compare the treatment plan with CDT, tooth, surface, quadrant, and benefit limits."
      ];
      await route.fulfill({
        status: 202,
        json: {
          data: {
            authorizationStatus: "Pending",
            serviceType: body.serviceType,
            incidentSeverity: body.incidentSeverity,
            dentalService: body.dentalService,
            review: "queued",
            ...(isDentalPredetermination ? { requiredDocuments: dentalRequiredDocuments, manualReviewPrompts: dentalManualReviewPrompts } : {})
          },
          transaction: {
            ...demoTransactions.find((transaction) => transaction.type === "278"),
            id: "tx-e2e-auth-review",
            status: "Pending",
            payload: {
              x12: isDentalPredetermination ? "278 dental predetermination fixture" : "278 resurrection auth fixture",
              adventurerId: body.adventurerId,
              providerId: body.providerId,
              serviceType: body.serviceType,
              dentalService: body.dentalService,
              ...(isDentalPredetermination ? { requiredDocuments: dentalRequiredDocuments, manualReviewPrompts: dentalManualReviewPrompts } : {})
            },
            rawX12: isDentalPredetermination
              ? "ST*278*dental~UM*AR*I*2***dental-predetermination~SV1*AD:D7240*0.00*UN*1~TOO*JP*14~SE*5*dental~"
              : demoTransactions.find((transaction) => transaction.type === "278")!.rawX12
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
      const body = route.request().postDataJSON() as {
        packetId?: string;
        attachments?: Array<{
          attachmentType: string;
          reportTypeCode: string;
          documentReferenceId: string;
          documentReferenceUrl: string;
          description: string;
        }>;
      };
      if (body.attachments?.length) {
        const attachmentTemplate = demoTransactions.find((transaction) => transaction.type === "275");
        expect(attachmentTemplate).toBeTruthy();
        const packetId = body.packetId ?? "auth-packet-e2e";
        const transactions = body.attachments.map((attachment, index) => ({
          ...attachmentTemplate!,
          id: `tx-e2e-auth-doc-${index + 1}`,
          status: "Accepted",
          relatedId: "tx-e2e-auth-review",
          payload: {
            x12: "275 dashboard auth packet fixture",
            authorizationTransactionId: "tx-e2e-auth-review",
            adventurerId: "adv-e2e-dashboard",
            attachmentType: attachment.attachmentType,
            reportTypeCode: attachment.reportTypeCode,
            packetId,
            packetSequence: String(index + 1),
            packetCount: String(body.attachments?.length ?? 1),
            attachmentReviewStatus: "Received",
            documentReferenceId: attachment.documentReferenceId,
            documentReferenceUrl: attachment.documentReferenceUrl,
            description: attachment.description
          }
        }));
        workbenchTransactions.push(...transactions);
        await route.fulfill({
          status: 201,
          json: {
            data: { authorizationTransactionId: "tx-e2e-auth-review", packetId, attachmentCount: transactions.length },
            lore: "Patient information attachment packet accepted for prior authorization.",
            transaction: transactions[0],
            transactions
          }
        });
        return;
      }
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

    if (path === "/v1/claims" && route.request().method() === "POST") {
      const body = route.request().postDataJSON() as {
        adventurerId: string;
        providerId: string;
        incidentSeverity: string;
        amountCents: number;
        serviceLines?: Array<{
          procedureCode?: string;
          cdtCode?: string;
          description?: string;
          amountCents?: number;
          toothNumber?: string;
          surface?: string;
          quadrant?: string;
          orthodontic?: boolean;
        }>;
      };
      const isDentalClaim = body.serviceLines?.some((line) => line.cdtCode || line.toothNumber || line.surface || line.quadrant || line.orthodontic) ?? false;
      const claimTransaction = demoTransactions.find((transaction) => transaction.type === (isDentalClaim ? "837D" : "837"));
      await route.fulfill({
        status: 201,
        json: {
          data: {
            id: "claim-e2e-dashboard",
            adventurerId: body.adventurerId,
            providerId: body.providerId,
            incidentSeverity: body.incidentSeverity,
            transactionId: claimTransaction?.id ?? "tx-e2e-837",
            amountCents: body.amountCents,
            allowedAmountCents: 80000,
            paidAmountCents: 70400,
            patientResponsibilityCents: 9600,
            adjustmentAmountCents: 20000,
            adjustmentReason: "ASHN contractual allowance with current premium",
            serviceLines: body.serviceLines,
            status: claimStatus
          },
          lore: isDentalClaim ? "Dental claim submitted." : "Scenario claim submitted.",
          transaction: claimTransaction,
          transactions: [
            claimTransaction,
            demoTransactions.find((transaction) => transaction.type === "277CA")
          ].filter(Boolean)
        }
      });
      return;
    }

    if (path === "/v1/claims/claim-e2e-dashboard/payment") {
      claimStatus = "Paid";
      await route.fulfill({
        status: 200,
        json: {
          data: {
            id: "claim-e2e-dashboard",
            adventurerId: "adv-e2e-dashboard",
            providerId: "provider-vitesse-temple",
            incidentSeverity: "Normal",
            transactionId: "tx-e2e-837d",
            amountCents: 85000,
            allowedAmountCents: 76500,
            paidAmountCents: 68850,
            patientResponsibilityCents: 7650,
            adjustmentAmountCents: 8500,
            adjustmentReason: "Dental plan allowance",
            status: claimStatus,
            serviceLines: [
              {
                lineNumber: 1,
                procedureCode: "D7240",
                cdtCode: "D7240",
                amountCents: 85000,
                allowedAmountCents: 76500,
                paidAmountCents: 68850,
                patientResponsibilityCents: 7650,
                adjustmentAmountCents: 8500,
                toothNumber: "14",
                surface: "MO",
                quadrant: "UR"
              }
            ]
          },
          lore: "Dental claim paid.",
          transaction: {
            ...demoTransactions.find((transaction) => transaction.type === "835"),
            id: "tx-e2e-835-dental",
            status: "Paid",
            payload: {
              x12: "835 Dental Claim Payment / Remittance Advice",
              claimId: "claim-e2e-dashboard",
              serviceLines: [{ cdtCode: "D7240", toothNumber: "14", allowedAmountCents: 76500, paidAmountCents: 68850 }]
            },
            rawX12: "ST*835*dental~CLP*claim-e2e-dashboard*1*850.00*688.50*76.50~SVC*AD:D7240*850.00*688.50~AMT*AU*765.00~AMT*PR*76.50~REF*XZ*TOOTH-14~SE*7*dental~"
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
      const body = route.request().postDataJSON() as { reason?: string; requiredDocuments?: unknown[] } | null;
      const requiredDocumentCount = body?.requiredDocuments?.length ?? 3;
      await route.fulfill({
        status: 202,
        json: {
          data: { claimId: "claim-e2e-dashboard", status: claimStatus, requestedTransaction: "275", requiredDocumentCount },
          transaction: {
            ...demoTransactions.find((transaction) => transaction.type === "277"),
            id: requiredDocumentCount === 1 ? "tx-e2e-deficiency-request" : "tx-e2e-doc-request",
            relatedId: "tx-e2e-837"
          }
        }
      });
      return;
    }

    if (path === "/v1/claims/claim-e2e-dashboard/attachments") {
      claimStatus = "Pending";
      const packet = route.request().postDataJSON() as {
        packetId: string;
        attachments: Array<{
          attachmentType: string;
          reportTypeCode: string;
          documentReferenceId: string;
          documentReferenceUrl: string;
          description: string;
        }>;
      };
      const attachmentTemplate = demoTransactions.find((transaction) => transaction.type === "275");
      expect(attachmentTemplate).toBeTruthy();
      const baseIndex = workbenchTransactions.length;
      const transactions = packet.attachments.map((attachment, index) => ({
        ...attachmentTemplate!,
        id: `tx-e2e-doc-packet-${baseIndex + index + 1}`,
        relatedId: "tx-e2e-837",
        payload: {
          x12: "275 documentation workbench packet fixture",
          claimId: "claim-e2e-dashboard",
          attachmentType: attachment.attachmentType,
          reportTypeCode: attachment.reportTypeCode,
          packetId: packet.packetId,
          packetSequence: String(index + 1),
          packetCount: String(packet.attachments.length),
          attachmentReviewStatus: "Received",
          documentReferenceId: attachment.documentReferenceId,
          documentReferenceUrl: attachment.documentReferenceUrl,
          description: attachment.description
        }
      }));
      if (baseIndex === 0) {
        workbenchTransactions.splice(0, workbenchTransactions.length, ...transactions);
      } else {
        workbenchTransactions.push(...transactions);
      }
      await route.fulfill({
        status: 201,
        json: {
          data: { claimId: "claim-e2e-dashboard", claimStatus, packetId: packet.packetId, attachmentCount: transactions.length },
          lore: "Patient information attachment accepted for documentation workbench.",
          transaction: transactions[0],
          transactions
        }
      });
      return;
    }

    const packetReviewMatch = path.match(/^\/v1\/transactions\/(tx-e2e-doc-packet-\d+)\/attachment-review$/);
    if (packetReviewMatch) {
      const transaction = workbenchTransactions.find((item) => item.id === packetReviewMatch[1]);
      const body = route.request().postDataJSON() as { status: string; reason: string };
      if (transaction) {
        transaction.payload = {
          ...transaction.payload,
          attachmentReviewStatus: body.status,
          attachmentReviewReason: body.reason
        };
      }
      await route.fulfill({
        json: {
          data: { transactionId: packetReviewMatch[1], attachmentReviewStatus: body.status, reason: body.reason },
          lore: `Attachment review marked ${body.status.toLowerCase()}.`,
          transaction
        }
      });
      return;
    }

    const authPacketReviewMatch = path.match(/^\/v1\/transactions\/(tx-e2e-auth-doc-\d+)\/attachment-review$/);
    if (authPacketReviewMatch) {
      const body = route.request().postDataJSON() as { status: string; reason?: string };
      const index = workbenchTransactions.findIndex((transaction) => transaction.id === authPacketReviewMatch[1]);
      expect(index).toBeGreaterThanOrEqual(0);
      const transaction = workbenchTransactions[index]!;
      workbenchTransactions[index] = {
        ...transaction,
        payload: {
          ...transaction.payload,
          attachmentReviewStatus: body.status,
          attachmentReviewReason: body.reason ?? ""
        }
      };
      await route.fulfill({
        json: {
          data: { transactionId: authPacketReviewMatch[1], attachmentReviewStatus: body.status, reason: body.reason },
          transaction: workbenchTransactions[index],
          lore: `Attachment review marked ${body.status.toLowerCase()}.`
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
      const allTransactions = [...workbenchTransactions, ...demoTransactions];
      const data = type ? allTransactions.filter((transaction) => transaction.type === type) : allTransactions;
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

    if (path === "/v1/transactions/tx-e2e-275/document-reference") {
      await route.fulfill({
        json: {
          data: {
            transactionId: "tx-e2e-275",
            claimId: "claim-e2e-275",
            attachmentType: "OZ",
            attachmentControlNumber: "ATTACH-E2E-275",
            reportTypeCode: "B4",
            contentType: "application/pdf",
            description: "E2E vault document",
            documentReferenceId: "doc-e2e-275",
            documentReferenceUrl: "https://docs.example.test/doc-e2e-275.pdf",
            embeddedContentAvailable: false,
            retrievalMode: "https",
            retrievalStatus: "external-reference",
            retrievalInstructions: "Use authorized document-vault credentials."
          },
          lore: "The Society document vault resolved the 275 reference without fetching external scrolls."
        }
      });
      return;
    }

    if (path === "/v1/x12/raw") {
      expect(route.request().headers()["content-type"]).toContain("application/edi-x12");
      const rawPayload = route.request().postData() ?? "";
      const rejectionError = raw275RejectionError(rawPayload);
      if (rejectionError) {
        const message: DemoInboundMessage = {
          id: `msg-e2e-275-${rejected275Messages.length + 1}`,
          partnerId: "provider-vitesse-temple",
          contentType: "application/edi-x12",
          transactionType: "275",
          rawPayload,
          status: "rejected",
          error: rejectionError,
          downstreamStatus: 400,
          createdAt: new Date(Date.UTC(2026, 6, 8, 14, rejected275Messages.length, 0)).toISOString()
        };
        rejected275Messages.unshift(message);
        await route.fulfill({
          status: 400,
          json: {
            error: rejectionError,
            lore: "The 275 fixture was rejected and preserved in intake audit.",
            data: { messageId: message.id, transactionType: "275" }
          }
        });
        return;
      }
      if (rawPayload.includes("ST*834")) {
        await route.fulfill({
          status: 201,
          json: {
            data: { id: "adv-e2e-raw-834", name: "Raw Enrollee", coverageStatus: "Active" },
            lore: "Raw X12 enrollment accepted.",
            transaction: {
              ...demoTransactions.find((transaction) => transaction.type === "834"),
              id: "tx-e2e-raw-834",
              payload: { x12: "834 raw dashboard intake fixture", name: "Raw Enrollee" }
            }
          }
        });
        return;
      }
      if (rawPayload.includes("ST*820")) {
        await route.fulfill({
          status: 201,
          json: {
            data: { adventurerId: "adv-e2e-dashboard", amountCents: 5000, status: "Accepted" },
            lore: "Raw X12 premium accepted.",
            transaction: {
              ...demoTransactions.find((transaction) => transaction.type === "820"),
              id: "tx-e2e-raw-820",
              payload: { x12: "820 raw dashboard intake fixture", amountCents: 5000 }
            }
          }
        });
        return;
      }
      if (rawPayload.includes("ST*270")) {
        await route.fulfill({
          status: 200,
          json: {
            data: { eligible: true },
            lore: "Raw X12 eligibility checked.",
            transaction: {
              ...demoTransactions.find((transaction) => transaction.type === "271"),
              id: "tx-e2e-raw-270",
              payload: { x12: "270 raw dashboard intake fixture", adventurerId: "adv-e2e-dashboard" }
            }
          }
        });
        return;
      }
      if (rawPayload.includes("ST*276")) {
        await route.fulfill({
          status: 200,
          json: {
            data: { claimId: "claim-e2e-dashboard", status: "Paid" },
            lore: "Raw X12 claim status checked.",
            transaction: {
              ...demoTransactions.find((transaction) => transaction.type === "277"),
              id: "tx-e2e-raw-276",
              payload: { x12: "276 raw dashboard intake fixture", claimId: "claim-e2e-dashboard" }
            }
          }
        });
        return;
      }
      if (rawPayload.includes("ST*278")) {
        await route.fulfill({
          status: 202,
          json: {
            data: { transactionId: "tx-e2e-raw-278", status: "Pending" },
            lore: "Raw X12 prior authorization queued.",
            transaction: {
              ...demoTransactions.find((transaction) => transaction.type === "278"),
              id: "tx-e2e-raw-278",
              payload: { x12: "278 raw dashboard intake fixture", serviceType: "resurrection" }
            }
          }
        });
        return;
      }
      if (rawPayload.includes("ST*835")) {
        await route.fulfill({
          status: 200,
          json: {
            data: { id: "claim-e2e-dashboard", status: "Paid" },
            lore: "Raw X12 payment accepted.",
            transaction: {
              ...demoTransactions.find((transaction) => transaction.type === "835"),
              id: "tx-e2e-raw-835",
              payload: { x12: "835 raw dashboard intake fixture", claimId: "claim-e2e-dashboard" }
            }
          }
        });
        return;
      }
      await route.fulfill({
        status: 201,
        json: {
          data: { id: "claim-e2e-raw", status: "Submitted" },
          lore: "Raw X12 claim submitted.",
          transaction: {
            ...demoTransactions.find((transaction) => transaction.type === "837"),
            id: "tx-e2e-raw-837",
            payload: { x12: "837 raw dashboard intake fixture", claimId: "claim-e2e-raw" }
          }
        }
      });
      return;
    }

    if (path === "/v1/x12/batch") {
      expect(route.request().headers()["content-type"]).toContain("multipart/form-data");
      await route.fulfill({
        status: 207,
        json: {
          lore: "The intake file-drop processed its batch scrolls.",
          data: {
            total: 2,
            accepted: 1,
            rejected: 1,
            results: [
              { fileName: "eligibility.xml", contentType: "application/xml", statusCode: 201, accepted: true, transactionType: "270" },
              { fileName: "claim.x12", contentType: "application/edi-x12", statusCode: 400, accepted: false, error: "invalid raw X12" }
            ]
          }
        }
      });
      return;
    }

    if (path === "/v1/x12/messages/msg-e2e-rejected-837/replay") {
      await route.fulfill({
        status: 202,
        json: {
          lore: "Replay queued for rejected 837 intake.",
          data: { id: "msg-e2e-rejected-837", status: "replay-queued" }
        }
      });
      return;
    }

    if (path === "/v1/x12/messages/rejections") {
      const type = url.searchParams.get("type");
      const q = url.searchParams.get("q")?.toLowerCase() ?? "";
      const allMessages = [...rejected275Messages, ...demoInboundMessages];
      const rejected = allMessages.filter((message) => {
        const searchable = `${message.id} ${message.partnerId ?? ""} ${message.transactionType ?? ""} ${message.rawPayload} ${message.status} ${message.error ?? ""}`.toLowerCase();
        return message.status === "rejected" && (!type || message.transactionType === type) && (!q || searchable.includes(q));
      });
      await route.fulfill({ json: { data: rejectionMetrics(rejected) } });
      return;
    }

    const transactionMatch = path.match(/^\/v1\/transactions\/([^/]+)$/);
    if (transactionMatch) {
      const transaction = demoTransactions.find((item) => item.id === transactionMatch[1]);
      await route.fulfill({ status: transaction ? 200 : 404, json: transaction ? { data: transaction } : { error: "transaction not found" } });
      return;
    }

    if (path === "/v1/x12/messages") {
      const status = url.searchParams.get("status");
      const type = url.searchParams.get("type");
      const q = url.searchParams.get("q")?.toLowerCase() ?? "";
      const allMessages = [...rejected275Messages, ...demoInboundMessages];
      const data = allMessages.filter((message) => {
        const searchable = `${message.id} ${message.partnerId ?? ""} ${message.transactionType ?? ""} ${message.rawPayload} ${message.status} ${message.error ?? ""}`.toLowerCase();
        const matchesStatus = !status || status === "All" || message.status === status;
        const matchesType = !type || type === "All" || message.transactionType === type;
        const matchesSearch = !q || searchable.includes(q);
        return matchesStatus && matchesType && matchesSearch;
      });
      await route.fulfill({ json: { data, page: pageInfo(data.length, 10) } });
      return;
    }

    await route.fulfill({ status: 404, json: { error: `unmocked route ${path}` } });
  });
}

function rejectionMetrics(messages: DemoInboundMessage[]) {
  const rejected = messages.filter((message) => message.status === "rejected");
  return {
    total: rejected.length,
    byPartner: topCounts(rejected.map((message) => ({ label: message.partnerId ?? "Unknown partner", query: message.partnerId ?? "Unknown partner", partnerId: message.partnerId }))),
    byType: topCounts(rejected.map((message) => ({ label: message.transactionType ?? "Unknown type", type: message.transactionType }))),
    byReason: topCounts(rejected.map((message) => ({ label: rejectionReason(message.error), query: "diagnosis code" }))),
    trend: topCounts(rejected.map((message) => ({ label: message.createdAt.slice(0, 10) }))).map((item) => ({ date: item.label, count: item.count })).sort((left, right) => left.date.localeCompare(right.date)),
    latest: rejected.slice(0, 5)
  };
}

function topCounts(items: Array<{ label: string; query?: string; type?: string; partnerId?: string }>) {
  const counts = new Map<string, { label: string; count: number; query?: string; type?: string; partnerId?: string }>();
  for (const item of items) {
    const current = counts.get(item.label) ?? { ...item, count: 0 };
    current.count += 1;
    counts.set(item.label, current);
  }
  return [...counts.values()].sort((left, right) => right.count - left.count || left.label.localeCompare(right.label));
}

function rejectionReason(error?: string) {
  const text = (error ?? "").toLowerCase();
  if (text.includes("diagnosis code")) return "Diagnosis code profile";
  if (text.includes("attachment purpose")) return "Attachment purpose profile";
  if (text.includes("attachment format")) return "Attachment format profile";
  if (text.includes("base64") || text.includes("mime")) return "Attachment payload encoding";
  if (text.includes("lx loops") || text.includes("packet contains")) return "Attachment packet limit";
  if (text.includes("solicited attachment") || text.includes("trace")) return "Solicited trace matching";
  if (text.includes("same day") || (text.includes("within") && text.includes("unsolicited"))) return "Late unsolicited attachment";
  return "Unknown rejection";
}

function raw275RejectionError(rawPayload: string) {
  if (!rawPayload.includes("ST*275")) return "";
  if (rawPayload.includes("BGN*99")) return "attachment purpose must be solicited or unsolicited";
  if (rawPayload.includes("CAT*B4*BIN")) return "attachment format BIN is not allowed for provider provider-vitesse-temple; allowed: TXT";
  if (rawPayload.includes("ATTACH-TOO-MANY")) return "attachment packet contains 6 LX loops; provider provider-vitesse-temple allows 5";
  if (rawPayload.includes("ATTACH-BAD-B64")) return "B64 attachment content must be valid base64";
  if (rawPayload.includes("ATTACH-MISSING-TRACE")) return "solicited attachment must include attachmentTraceId tx-doc-request";
  if (rawPayload.includes("ATTACH-LATE")) return "unsolicited 275 attachments for provider provider-vitesse-temple must be submitted on the same day as the originating 837 claim";
  return "";
}

function pageInfo(count: number, limit: number) {
  return {
    limit,
    offset: 0,
    count,
    hasMore: false
  };
}
