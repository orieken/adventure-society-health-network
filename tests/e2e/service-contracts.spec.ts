import { expect, test } from "@orieken/saturday-playwright";

import { runMutatingE2E, services, serviceUrls, uniqueDemoName } from "./config.js";

type Envelope<T = unknown> = {
  data?: T;
  error?: string;
  lore?: string;
  page?: {
    limit: number;
    offset: number;
    count: number;
    hasMore: boolean;
  };
  transaction?: {
    id: string;
    type: string;
    status: string;
  };
  transactions?: Array<{
    id: string;
    type: string;
    status: string;
  }>;
};

test.describe("ASHN service contracts", () => {
  for (const service of services) {
    test(`${service.label} publishes OpenAPI docs`, async ({ request }) => {
      const root = await request.get(`${service.baseURL}/`);
      expect(root.ok()).toBeTruthy();
      await expect(await root.text()).toContain("OpenAPI JSON");

      const response = await request.get(`${service.baseURL}/openapi.json`);
      expect(response.ok()).toBeTruthy();

      const spec = (await response.json()) as {
        openapi?: string;
        info?: { title?: string; version?: string };
        paths?: Record<string, unknown>;
      };

      expect(spec.openapi).toBe("3.1.0");
      expect(spec.info?.title).toContain("ASHN");
      expect(spec.info?.version).toBeTruthy();
      expect(Object.keys(spec.paths ?? {}).length).toBeGreaterThan(0);
    });

    test(`${service.label} health endpoint follows the service contract`, async ({ request }) => {
      const response = await request.get(`${service.baseURL}${service.healthPath}`);
      expect(response.ok()).toBeTruthy();

      const body = await response.json();
      if (service.key === "apiGateway") {
        const envelope = body as Envelope<Record<string, string>>;
        expect(envelope.data?.[service.expectedService]).toBe("ok");
        expect(envelope.data?.["payer-core"]).toMatch(/ok|unknown|unavailable/);
        expect(envelope.data?.["provider-service"]).toMatch(/ok|unknown|unavailable/);
        expect(envelope.data?.["edi-intake"]).toMatch(/ok|unknown|unavailable/);
      } else {
        expect(body).toMatchObject({ status: "ok", service: service.expectedService });
      }
    });
  }

  test("gateway lists transactions with server-side pagination", async ({ request }) => {
    const response = await request.get(`${serviceUrls.apiGateway}/v1/transactions?limit=5&offset=0`);
    expect(response.ok()).toBeTruthy();

    const envelope = (await response.json()) as Envelope<unknown[]>;
    expect(Array.isArray(envelope.data)).toBeTruthy();
    expect(envelope.page?.limit).toBe(5);
    expect(envelope.page?.offset).toBe(0);
    expect(typeof envelope.page?.count).toBe("number");
    expect(typeof envelope.page?.hasMore).toBe("boolean");
  });
});

test.describe("ASHN mutating demo contracts", () => {
  test.skip(!runMutatingE2E, "Set ASHN_RUN_MUTATING_E2E=1 to run tests that create demo ledger data.");

  test("gateway supports enrollment, eligibility, claim, and ledger lookup", async ({ request }) => {
    const enrollment = await request.post(`${serviceUrls.apiGateway}/v1/adventurers`, {
      data: {
        name: uniqueDemoName("Playwright Paladin"),
        rank: "Gold",
        guild: "Contract Testers Guild",
        region: "Rimaros"
      }
    });
    expect(enrollment.status()).toBe(201);

    const enrolled = (await enrollment.json()) as Envelope<{ id: string; coverageStatus: string }>;
    expect(enrolled.data?.id).toBeTruthy();
    expect(enrolled.data?.coverageStatus).toBe("active");
    expect(enrolled.transaction?.type).toBe("834");

    const eligibility = await request.post(`${serviceUrls.apiGateway}/v1/eligibility`, {
      data: {
        adventurerId: enrolled.data?.id,
        providerId: "provider-vitesse-temple"
      }
    });
    expect(eligibility.ok()).toBeTruthy();

    const eligibilityEnvelope = (await eligibility.json()) as Envelope<{ eligible: boolean }>;
    expect(eligibilityEnvelope.data?.eligible).toBe(true);
    expect(eligibilityEnvelope.transactions?.map((transaction) => transaction.type)).toEqual(["270", "271"]);

    const claim = await request.post(`${serviceUrls.apiGateway}/v1/claims`, {
      data: {
        adventurerId: enrolled.data?.id,
        providerId: "provider-vitesse-temple",
        incidentSeverity: "dragonfire",
        amountCents: 125_00
      }
    });
    expect(claim.status()).toBe(201);

    const claimEnvelope = (await claim.json()) as Envelope<{ id: string; status: string }>;
    expect(claimEnvelope.data?.id).toBeTruthy();
    expect(claimEnvelope.transaction?.type).toBe("837");
    expect(claimEnvelope.transactions?.map((transaction) => transaction.type)).toEqual(["837", "277CA"]);

    const ledger = await request.get(`${serviceUrls.apiGateway}/v1/transactions?limit=5&type=837`);
    expect(ledger.ok()).toBeTruthy();

    const ledgerEnvelope = (await ledger.json()) as Envelope<Array<{ id: string; type: string }>>;
    expect(ledgerEnvelope.data?.some((transaction) => transaction.type === "837")).toBeTruthy();
  });

  test("gateway accepts XML intake and exposes audit visibility", async ({ request }) => {
    const xmlName = uniqueDemoName("XML Ranger");
    const xml = `<?xml version="1.0" encoding="UTF-8"?>
<AshnX12Transaction type="834">
  <Sender id="guild-admin"/>
  <Receiver id="adventure-society"/>
  <Enrollment>
    <Name>${xmlName}</Name>
    <Rank>Silver</Rank>
    <Guild>XML Demo Guild</Guild>
    <Region>Greenstone</Region>
  </Enrollment>
</AshnX12Transaction>`;

    const intake = await request.post(`${serviceUrls.apiGateway}/v1/x12/xml`, {
      headers: {
        "Content-Type": "application/xml"
      },
      data: xml
    });
    expect(intake.status()).toBe(201);

    const messages = await request.get(`${serviceUrls.apiGateway}/v1/x12/messages?limit=5&type=834&q=${encodeURIComponent(xmlName)}`);
    expect(messages.ok()).toBeTruthy();

    const envelope = (await messages.json()) as Envelope<Array<{ id: string; transactionType: string; status: string }>>;
    expect(envelope.data?.some((message) => message.transactionType === "834" && message.status === "accepted")).toBeTruthy();
  });
});
