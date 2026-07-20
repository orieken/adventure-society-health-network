import { expect, test } from "@orieken/saturday-playwright";
import type { APIRequestContext } from "@playwright/test";

import { expectOperationsE2E, runMutatingE2E, services, serviceUrls, uniqueDemoName } from "./config.js";

type TransactionSummary = {
  id: string;
  type: string;
  status: string;
  relatedId?: string;
  payload?: {
    adjudication?: {
      paidAmountCents?: number;
      patientResponsibilityCents?: number;
      adjustmentReason?: string;
      premiumCurrent?: boolean;
      premiumPaidAmountCents?: number;
    };
  };
};

type ClaimDetail = {
  id: string;
  status: string;
  allowedAmountCents?: number;
  paidAmountCents?: number;
  patientResponsibilityCents?: number;
  adjustmentReason?: string;
};

type Adventurer = {
  id: string;
  coverageStatus: string;
};

type InboundMessage = {
  id: string;
  contentType: string;
  transactionType: string;
  status: string;
  error?: string;
};

type TradingPartner = {
  id: string;
  name: string;
  senderId: string;
  receiverId: string;
  allowedTransactionTypes: string[];
  routeTarget: string;
  status: string;
};

type DocumentReference = {
  transactionId: string;
  retrievalMode: string;
  retrievalStatus: string;
  embeddedContentAvailable: boolean;
  documentReferenceUrl?: string;
};

type ReadinessReport = {
  generatedAt: string;
  status: string;
  services: Array<{ service: string; status: string }>;
  checks: Array<{ name: string; status: string }>;
};

type MetricsSummary = {
  generatedAt: string;
  operationalStatus: string;
  transactions: {
    totalLoaded: number;
    byType: Record<string, number>;
    byStatus: Record<string, number>;
  };
  claims: {
    totalLoaded: number;
    byStatus: Record<string, number>;
    byProvider: Record<string, number>;
  };
  financials: {
    billedCents: number;
    allowedCents: number;
    paidCents: number;
    patientResponsibilityCents: number;
    adjustmentCents: number;
  };
  intake: {
    rejectionTotal: number;
    byPartner: Record<string, number>;
    byType: Record<string, number>;
    byReason: Record<string, number>;
  };
  asyncJobs: {
    totalLoaded: number;
    byStatus: Record<string, number>;
    deadLetters: number;
  };
  highlights: string[];
};

type TransactionJob = {
  id: string;
  type: string;
  status: string;
  attempts: number;
  deadLetter: boolean;
};

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
  transaction?: TransactionSummary;
  transactions?: TransactionSummary[];
};

async function enrollAdventurer(request: APIRequestContext, namePrefix: string) {
  const enrollment = await request.post(`${serviceUrls.apiGateway}/v1/adventurers`, {
    data: {
      name: uniqueDemoName(namePrefix),
      rank: "Gold",
      guild: "Contract Testers Guild",
      region: "Rimaros"
    }
  });
  expect(enrollment.status()).toBe(201);

  const enrolled = (await enrollment.json()) as Envelope<Adventurer>;
  expect(enrolled.data?.id).toBeTruthy();
  expect(enrolled.data?.coverageStatus).toBe("Active");
  expect(enrolled.transaction?.type).toBe("834");
  return enrolled;
}

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

  test("gateway propagates request and correlation headers", async ({ request }) => {
    const requestId = `req-e2e-${Date.now()}`;
    const correlationId = `corr-e2e-${Date.now()}`;

    const response = await request.get(`${serviceUrls.apiGateway}/v1/health`, {
      headers: {
        "X-Request-ID": requestId,
        "X-Correlation-ID": correlationId
      }
    });
    expect(response.ok()).toBeTruthy();
    expect(response.headers()["x-request-id"]).toBe(requestId);
    expect(response.headers()["x-correlation-id"]).toBe(correlationId);
    expect(response.headers()["traceparent"]).toMatch(/^00-/);
  });
});

test.describe("ASHN operations contracts", () => {
  test.skip(!expectOperationsE2E, "Set ASHN_EXPECT_OPERATIONS_E2E=1 after readiness and metrics endpoints are deployed.");

  test("gateway exposes system readiness signals", async ({ request }) => {
    const response = await request.get(`${serviceUrls.apiGateway}/v1/system/readiness`);
    expect(response.ok()).toBeTruthy();

    const envelope = (await response.json()) as Envelope<ReadinessReport>;
    expect(envelope.data?.generatedAt).toBeTruthy();
    expect(envelope.data?.status).toMatch(/ready|degraded|unavailable/);
    expect(envelope.data?.services.map((service) => service.service)).toEqual(
      expect.arrayContaining(["api-gateway", "payer-core", "provider-service", "edi-intake"])
    );
    expect(envelope.data?.checks.map((check) => check.name)).toEqual(
      expect.arrayContaining(["transaction-ledger", "async-worker-queue", "provider-registry", "xml-intake-audit"])
    );
  });

  test("gateway exposes operational metrics summary", async ({ request }) => {
    const response = await request.get(`${serviceUrls.apiGateway}/v1/metrics/summary`);
    expect(response.ok()).toBeTruthy();

    const envelope = (await response.json()) as Envelope<MetricsSummary>;
    expect(envelope.data?.generatedAt).toBeTruthy();
    expect(envelope.data?.operationalStatus).toMatch(/ready|degraded|unavailable/);
    expect(typeof envelope.data?.transactions.totalLoaded).toBe("number");
    expect(typeof envelope.data?.claims.totalLoaded).toBe("number");
    expect(typeof envelope.data?.financials.billedCents).toBe("number");
    expect(typeof envelope.data?.intake.rejectionTotal).toBe("number");
    expect(typeof envelope.data?.asyncJobs.deadLetters).toBe("number");
    expect(Array.isArray(envelope.data?.highlights)).toBeTruthy();
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
    expect(enrolled.data?.coverageStatus).toBe("Active");
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

    const auth = await request.post(`${serviceUrls.apiGateway}/v1/auth-requests`, {
      data: {
        adventurerId: enrolled.data?.id,
        providerId: "provider-vitesse-temple",
        serviceType: "resurrection",
        incidentSeverity: "Diamond"
      }
    });
    expect(auth.status()).toBe(202);

    const authEnvelope = (await auth.json()) as Envelope;
    expect(authEnvelope.transaction?.type).toBe("278");
    expect(authEnvelope.transaction?.status).toBe("Pending");

    const authAttachment = await request.post(`${serviceUrls.apiGateway}/v1/auth-requests/${authEnvelope.transaction?.id}/attachments`, {
      data: {
        attachmentType: "OZ",
        attachmentControlNumber: `ATTACH-AUTH-${Date.now()}`,
        reportTypeCode: "B4",
        transmissionCode: "EL",
        contentType: "text/plain",
        description: "Prior authorization medical necessity notes",
        content: "Authorization includes encounter notes and healer attestation.",
        documentReferenceId: `doc-auth-${Date.now()}`,
        documentReferenceUrl: "https://docs.example.test/ashn/auth-notes.txt"
      }
    });
    expect(authAttachment.status()).toBe(201);

    const authAttachmentEnvelope = (await authAttachment.json()) as Envelope<{ authorizationTransactionId: string; attachmentType: string }>;
    expect(authAttachmentEnvelope.data?.authorizationTransactionId).toBe(authEnvelope.transaction?.id);
    expect(authAttachmentEnvelope.transaction?.type).toBe("275");
    expect(authAttachmentEnvelope.transaction?.relatedId).toBe(authEnvelope.transaction?.id);

    const attachmentReview = await request.post(`${serviceUrls.apiGateway}/v1/transactions/${authAttachmentEnvelope.transaction?.id}/attachment-review`, {
      data: {
        status: "Accepted",
        reason: "E2E attachment review accepted supporting documentation."
      }
    });
    expect(attachmentReview.ok()).toBeTruthy();

    const attachmentReviewEnvelope = (await attachmentReview.json()) as Envelope<{ attachmentReviewStatus: string }>;
    expect(attachmentReviewEnvelope.data?.attachmentReviewStatus).toBe("Accepted");
    expect(attachmentReviewEnvelope.transaction?.status).toBe("Accepted");

    const decision = await request.post(`${serviceUrls.apiGateway}/v1/auth-requests/${authEnvelope.transaction?.id}/decision`, {
      data: {
        decision: "Approved",
        reason: "E2E manual review approved resurrection medical necessity."
      }
    });
    expect(decision.ok()).toBeTruthy();

    const decisionEnvelope = (await decision.json()) as Envelope;
    expect(decisionEnvelope.transaction?.type).toBe("278");
    expect(decisionEnvelope.transaction?.status).toBe("Approved");

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

    const documentationRequest = await request.post(`${serviceUrls.apiGateway}/v1/claims/${claimEnvelope.data?.id}/documentation-request`);
    expect(documentationRequest.status()).toBe(202);

    const documentationEnvelope = (await documentationRequest.json()) as Envelope<{ claimId: string; status: string; requestedTransaction: string }>;
    expect(documentationEnvelope.data?.status).toBe("Pending Documentation");
    expect(documentationEnvelope.data?.requestedTransaction).toBe("275");
    expect(documentationEnvelope.transaction?.type).toBe("277");

    const attachment = await request.post(`${serviceUrls.apiGateway}/v1/claims/${claimEnvelope.data?.id}/attachments`, {
      data: {
        attachmentType: "OZ",
        attachmentControlNumber: `ATTACH-${Date.now()}`,
        reportTypeCode: "B4",
        transmissionCode: "EL",
        contentType: "text/plain",
        description: "Resurrection medical necessity notes",
        content: "Patient stabilized after dragonfire incident."
      }
    });
    expect(attachment.status()).toBe(201);

    const attachmentEnvelope = (await attachment.json()) as Envelope<{ claimId: string; attachmentType: string }>;
    expect(attachmentEnvelope.data?.claimId).toBe(claimEnvelope.data?.id);
    expect((attachmentEnvelope.data as { claimStatus?: string } | undefined)?.claimStatus).toBe("Pending");
    expect(attachmentEnvelope.transaction?.type).toBe("275");
    expect(attachmentEnvelope.transaction?.status).toBe("Accepted");

    const packetId = `packet-${Date.now()}`;
    const attachmentPacket = await request.post(`${serviceUrls.apiGateway}/v1/claims/${claimEnvelope.data?.id}/attachments`, {
      data: {
        packetId,
        attachments: [
          {
            attachmentType: "OZ",
            attachmentControlNumber: `ATTACH-PKT-${Date.now()}-1`,
            reportTypeCode: "B4",
            transmissionCode: "EL",
            contentType: "text/plain",
            description: "Packet note one",
            content: "First packeted support note."
          },
          {
            attachmentType: "OZ",
            attachmentControlNumber: `ATTACH-PKT-${Date.now()}-2`,
            reportTypeCode: "B4",
            transmissionCode: "EL",
            contentType: "text/plain",
            description: "Packet note two",
            documentReferenceUrl: "https://docs.example.test/packet-note-two.txt"
          }
        ]
      }
    });
    expect(attachmentPacket.status()).toBe(201);
    const attachmentPacketEnvelope = (await attachmentPacket.json()) as Envelope<{ claimId: string; packetId: string; attachmentCount: number }>;
    expect(attachmentPacketEnvelope.data?.packetId).toBe(packetId);
    expect(attachmentPacketEnvelope.data?.attachmentCount).toBe(2);
    expect(attachmentPacketEnvelope.transactions?.map((transaction) => transaction.type)).toEqual(["275", "275"]);

    const xmlAttachment = `<?xml version="1.0" encoding="UTF-8"?>
<AshnX12Transaction type="275">
  <Sender id="provider-vitesse-temple"/>
  <Receiver id="Adventure Society"/>
  <Attachment>
    <ClaimId>${claimEnvelope.data?.id}</ClaimId>
    <ProviderId>provider-vitesse-temple</ProviderId>
    <AttachmentType>OZ</AttachmentType>
    <AttachmentControlNumber>XML-${Date.now()}</AttachmentControlNumber>
    <ReportTypeCode>B4</ReportTypeCode>
    <TransmissionCode>EL</TransmissionCode>
    <ContentType>text/plain</ContentType>
    <Description>Resurrection operative notes</Description>
    <Content>Dragonfire injury required restorative magic.</Content>
  </Attachment>
</AshnX12Transaction>`;

    const xmlIntake = await request.post(`${serviceUrls.apiGateway}/v1/x12/xml`, {
      headers: {
        "Content-Type": "application/xml"
      },
      data: xmlAttachment
    });
    expect(xmlIntake.status()).toBe(201);

    const rawControl = String(Date.now()).slice(-9).padStart(9, "0");
    const rawClaimId = `claim-raw-${rawControl}`;
    const rawX12 = [
      `ISA*00*          *00*          *ZZ*provider-vitesse-temple*ZZ*Adventure Society*260708*1200*^*00501*${rawControl}*0*T*:~`,
      `GS*HC*provider-vitesse-temple*Adventure Society*20260708*1200*${rawControl}*X*005010X837P~`,
      `ST*837*${rawControl}~`,
      `BHT*0019*00*${rawControl}*20260708*1200*CH~`,
      "HL*1**20*1~",
      "NM1*41*2*provider-vitesse-temple*****46*provider-vitesse-temple~",
      "NM1*85*2*provider-vitesse-temple*****XX*provider-vitesse-temple~",
      "HL*2*1*22*0~",
      `NM1*IL*1*${enrolled.data?.id}****MI*${enrolled.data?.id}~`,
      `CLM*${rawClaimId}*1250.00***11:B:1*Y*A*Y*I~`,
      "HI*ABK:S062X9A*ABF:T509~",
      "SV1*HC:ASHN1*950.00*UN*1***1~",
      "SV1*HC:ASHN2*300.00*UN*1***2~",
      `SE*13*${rawControl}~`,
      `GE*1*${rawControl}~`,
      `IEA*1*${rawControl}~`
    ].join("\n");
    const rawIntake = await request.post(`${serviceUrls.apiGateway}/v1/x12/raw`, {
      headers: {
        "Content-Type": "application/edi-x12"
      },
      data: rawX12
    });
    expect(rawIntake.status()).toBe(201);
    const rawEnvelope = (await rawIntake.json()) as Envelope<{ id: string }>;
    expect(rawEnvelope.transaction?.type).toBe("837");
    expect(rawEnvelope.data?.id).toBeTruthy();

    const rawClaim = await request.get(`${serviceUrls.apiGateway}/v1/claims/${rawEnvelope.data?.id}`);
    expect(rawClaim.ok()).toBeTruthy();
    const rawClaimEnvelope = (await rawClaim.json()) as Envelope<{
      id: string;
      diagnoses?: Array<{ qualifier: string; code: string; primary?: boolean }>;
      serviceLines?: Array<{ procedureCode: string; amountCents: number }>;
    }>;
    expect(rawClaimEnvelope.data?.diagnoses?.map((diagnosis) => diagnosis.code)).toEqual(["S062X9A", "T509"]);
    expect(rawClaimEnvelope.data?.diagnoses?.at(0)?.primary).toBe(true);
    expect(rawClaimEnvelope.data?.serviceLines?.map((line) => line.procedureCode)).toEqual(["ASHN1", "ASHN2"]);
    expect(rawClaimEnvelope.data?.serviceLines?.map((line) => line.amountCents)).toEqual([95000, 30000]);

    const ledger = await request.get(`${serviceUrls.apiGateway}/v1/transactions?limit=5&type=837`);
    expect(ledger.ok()).toBeTruthy();

    const ledgerEnvelope = (await ledger.json()) as Envelope<Array<{ id: string; type: string }>>;
    expect(ledgerEnvelope.data?.some((transaction) => transaction.type === "837")).toBeTruthy();

    const attachmentLedger = await request.get(`${serviceUrls.apiGateway}/v1/transactions?limit=5&type=275`);
    expect(attachmentLedger.ok()).toBeTruthy();

    const attachmentLedgerEnvelope = (await attachmentLedger.json()) as Envelope<Array<{ id: string; type: string; relatedId?: string }>>;
    expect(attachmentLedgerEnvelope.data?.some((transaction) => transaction.type === "275")).toBeTruthy();
  });

  test("gateway routes provider eligibility and claim workflow endpoints", async ({ request }) => {
    const enrolled = await enrollAdventurer(request, "Provider Portal Ranger");

    const eligibility = await request.post(`${serviceUrls.apiGateway}/v1/providers/provider-vitesse-temple/verify-eligibility`, {
      data: {
        adventurerId: enrolled.data?.id
      }
    });
    expect(eligibility.ok()).toBeTruthy();
    const eligibilityEnvelope = (await eligibility.json()) as Envelope<{ eligible: boolean; providerId: string }>;
    expect(eligibilityEnvelope.data).toMatchObject({ eligible: true, providerId: "provider-vitesse-temple" });
    expect(eligibilityEnvelope.transactions?.map((transaction) => transaction.type)).toEqual(["270", "271"]);

    const claim = await request.post(`${serviceUrls.apiGateway}/v1/providers/provider-vitesse-temple/submit-claim`, {
      data: {
        adventurerId: enrolled.data?.id,
        incidentSeverity: "Awakened",
        amountCents: 73_000
      }
    });
    expect(claim.status()).toBe(201);
    const claimEnvelope = (await claim.json()) as Envelope<{ id: string; providerId: string }>;
    expect(claimEnvelope.data).toMatchObject({ providerId: "provider-vitesse-temple" });
    expect(claimEnvelope.transaction?.type).toBe("837");
    expect(claimEnvelope.transactions?.map((transaction) => transaction.type)).toEqual(["837", "277CA"]);
  });

  test("gateway replays a ledger transaction into a related transaction", async ({ request }) => {
    const enrolled = await enrollAdventurer(request, "Replay Ledger Ranger");

    const claim = await request.post(`${serviceUrls.apiGateway}/v1/claims`, {
      data: {
        adventurerId: enrolled.data?.id,
        providerId: "provider-greenstone-roadside",
        incidentSeverity: "Normal",
        amountCents: 28_000
      }
    });
    expect(claim.status()).toBe(201);
    const claimEnvelope = (await claim.json()) as Envelope<{ id: string }>;
    expect(claimEnvelope.transaction?.id).toBeTruthy();

    const replay = await request.post(`${serviceUrls.apiGateway}/v1/transactions/${claimEnvelope.transaction?.id}/replay`);
    expect(replay.status()).toBe(201);
    const replayEnvelope = (await replay.json()) as Envelope<TransactionSummary>;
    expect(replayEnvelope.data?.type).toBe("837");
    expect(replayEnvelope.data?.relatedId).toBe(claimEnvelope.transaction?.id);
    expect(replayEnvelope.data?.id).not.toBe(claimEnvelope.transaction?.id);
  });

  test("gateway replays a dead-letter async job when one is available", async ({ request }) => {
    const jobsResponse = await request.get(`${serviceUrls.apiGateway}/v1/jobs?limit=50`);
    expect(jobsResponse.ok()).toBeTruthy();
    const jobsEnvelope = (await jobsResponse.json()) as Envelope<TransactionJob[]>;
    const deadLetter = jobsEnvelope.data?.find((job) => job.deadLetter);
    test.skip(!deadLetter, "No dead-letter async job is available to replay in this environment.");

    const replay = await request.post(`${serviceUrls.apiGateway}/v1/jobs/${deadLetter?.id}/replay`);
    expect(replay.status()).toBe(202);
    const replayEnvelope = (await replay.json()) as Envelope<TransactionJob>;
    expect(replayEnvelope.data).toMatchObject({
      id: deadLetter?.id,
      status: "pending",
      attempts: 0,
      deadLetter: false
    });
  });

  test("gateway applies recent 820 premium context during async claim adjudication", async ({ request }) => {
    const enrollment = await request.post(`${serviceUrls.apiGateway}/v1/adventurers`, {
      data: {
        name: uniqueDemoName("Premium Current Ranger"),
        rank: "Iron",
        guild: "Premium Contract Guild",
        region: "Greenstone"
      }
    });
    expect(enrollment.status()).toBe(201);

    const enrolled = (await enrollment.json()) as Envelope<{ id: string }>;
    expect(enrolled.data?.id).toBeTruthy();

    const premium = await request.post(`${serviceUrls.apiGateway}/v1/premium-payments`, {
      data: {
        adventurerId: enrolled.data?.id,
        amountCents: 25_00
      }
    });
    expect(premium.status()).toBe(201);

    const premiumEnvelope = (await premium.json()) as Envelope<{ status: string; amountCents: number }>;
    expect(premiumEnvelope.transaction?.type).toBe("820");
    expect(premiumEnvelope.data?.status).toBe("Accepted");

    const claim = await request.post(`${serviceUrls.apiGateway}/v1/claims`, {
      data: {
        adventurerId: enrolled.data?.id,
        providerId: "provider-greenstone-roadside",
        incidentSeverity: "Awakened",
        amountCents: 100_000
      }
    });
    expect(claim.status()).toBe(201);

    const claimEnvelope = (await claim.json()) as Envelope<ClaimDetail>;
    expect(claimEnvelope.data?.id).toBeTruthy();
    expect(claimEnvelope.transactions?.map((transaction) => transaction.type)).toEqual(["837", "277CA"]);

    await expect.poll(async () => {
      const response = await request.get(`${serviceUrls.apiGateway}/v1/claims/${claimEnvelope.data?.id}`);
      if (!response.ok()) {
        return "missing";
      }
      const current = (await response.json()) as Envelope<ClaimDetail>;
      return `${current.data?.status}:${current.data?.paidAmountCents ?? 0}`;
    }, {
      intervals: [1_000, 2_000, 3_000],
      timeout: 20_000
    }).toBe("Approved:70400");

    const finalizedClaimResponse = await request.get(`${serviceUrls.apiGateway}/v1/claims/${claimEnvelope.data?.id}`);
    const finalizedClaim = (await finalizedClaimResponse.json()) as Envelope<ClaimDetail>;
    expect(finalizedClaim.data?.allowedAmountCents).toBe(80_000);
    expect(finalizedClaim.data?.paidAmountCents).toBe(70_400);
    expect(finalizedClaim.data?.patientResponsibilityCents).toBe(9_600);
    expect(finalizedClaim.data?.adjustmentReason).toBe("ASHN contractual allowance with current premium");

    const ledger = await request.get(`${serviceUrls.apiGateway}/v1/transactions?limit=10&type=277&q=${encodeURIComponent(claimEnvelope.data?.id ?? "")}`);
    expect(ledger.ok()).toBeTruthy();

    const ledgerEnvelope = (await ledger.json()) as Envelope<TransactionSummary[]>;
    const adjudication277 = ledgerEnvelope.data?.find((transaction) => transaction.payload?.adjudication?.premiumCurrent);
    expect(adjudication277?.payload?.adjudication).toMatchObject({
      paidAmountCents: 70_400,
      patientResponsibilityCents: 9_600,
      adjustmentReason: "ASHN contractual allowance with current premium",
      premiumCurrent: true,
      premiumPaidAmountCents: 2_500
    });
  });

  test("gateway accepts XML intake and exposes audit visibility", async ({ request }) => {
    const xmlName = uniqueDemoName("XML Ranger");
    const xml = `<?xml version="1.0" encoding="UTF-8"?>
<AshnX12Transaction type="834">
  <Sender id="partner-greenstone"/>
  <Receiver id="Adventure Society"/>
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

  test("gateway rejects partner-specific 837 profile violations", async ({ request }) => {
    const invalidClaim = `<?xml version="1.0" encoding="UTF-8"?>
<AshnX12Transaction type="837">
  <Sender id="provider-vitesse-temple"/>
  <Receiver id="Adventure Society"/>
  <Claim>
    <AdventurerId>adv-profile-reject</AdventurerId>
    <ProviderId>provider-vitesse-temple</ProviderId>
    <IncidentSeverity>Awakened</IncidentSeverity>
    <AmountCents>10000</AmountCents>
    <Diagnosis qualifier="ABK" primary="true"><Code>M542</Code></Diagnosis>
    <ServiceLine lineNumber="1"><ProcedureCode>ASHN1</ProcedureCode><AmountCents>10000</AmountCents></ServiceLine>
  </Claim>
</AshnX12Transaction>`;

    const intake = await request.post(`${serviceUrls.apiGateway}/v1/x12/xml`, {
      headers: {
        "Content-Type": "application/xml"
      },
      data: invalidClaim
    });
    expect(intake.status()).toBe(400);
    const envelope = (await intake.json()) as Envelope;
    expect(envelope.error).toContain("diagnosis code M542 is not allowed");

    const messages = await request.get(`${serviceUrls.apiGateway}/v1/x12/messages?limit=5&type=837&q=M542`);
    expect(messages.ok()).toBeTruthy();
    const auditEnvelope = (await messages.json()) as Envelope<Array<{ transactionType: string; status: string; error?: string }>>;
    expect(auditEnvelope.data?.some((message) => message.transactionType === "837" && message.status === "rejected")).toBeTruthy();
  });

  test("gateway accepts canonical JSON intake through representation route", async ({ request }) => {
    const jsonName = uniqueDemoName("JSON Ranger");
    const intake = await request.post(`${serviceUrls.apiGateway}/v1/x12/transactions`, {
      headers: {
        "Content-Type": "application/vnd.ashn+x12+json"
      },
      data: {
        type: "834",
        sender: { id: "partner-greenstone" },
        receiver: { id: "Adventure Society" },
        enrollment: {
          name: jsonName,
          rank: "Silver",
          guild: "JSON Demo Guild",
          region: "Greenstone"
        }
      }
    });
    expect(intake.status()).toBe(201);

    const messages = await request.get(`${serviceUrls.apiGateway}/v1/x12/messages?limit=5&type=834&q=${encodeURIComponent(jsonName)}`);
    expect(messages.ok()).toBeTruthy();

    const envelope = (await messages.json()) as Envelope<Array<{ contentType: string; transactionType: string; status: string }>>;
    expect(envelope.data?.some((message) => message.contentType.includes("json") && message.transactionType === "834" && message.status === "accepted")).toBeTruthy();
  });

  test("gateway exports transactions and resolves embedded 275 document content", async ({ request }) => {
    const enrolled = await enrollAdventurer(request, "Export Cleric");

    const claim = await request.post(`${serviceUrls.apiGateway}/v1/claims`, {
      data: {
        adventurerId: enrolled.data?.id,
        providerId: "provider-vitesse-temple",
        incidentSeverity: "dragonfire",
        amountCents: 42_000
      }
    });
    expect(claim.status()).toBe(201);

    const claimEnvelope = (await claim.json()) as Envelope<{ id: string }>;
    expect(claimEnvelope.transaction?.id).toBeTruthy();

    const x12Export = await request.get(`${serviceUrls.apiGateway}/v1/transactions/${claimEnvelope.transaction?.id}/export?format=x12`);
    expect(x12Export.ok()).toBeTruthy();
    expect(x12Export.headers()["content-type"]).toContain("text/plain");
    await expect(await x12Export.text()).toContain("ST*837");

    const xmlExport = await request.get(`${serviceUrls.apiGateway}/v1/transactions/${claimEnvelope.transaction?.id}/export?format=xml`);
    expect(xmlExport.ok()).toBeTruthy();
    expect(xmlExport.headers()["content-type"]).toContain("application/xml");
    await expect(await xmlExport.text()).toContain("<AshnTransactionExport");

    const jsonExport = await request.get(`${serviceUrls.apiGateway}/v1/transactions/${claimEnvelope.transaction?.id}/export?format=json`);
    expect(jsonExport.ok()).toBeTruthy();
    expect(jsonExport.headers()["content-type"]).toContain("application/json");
    const exportedTransaction = (await jsonExport.json()) as TransactionSummary;
    expect(exportedTransaction.type).toBe("837");

    const attachment = await request.post(`${serviceUrls.apiGateway}/v1/claims/${claimEnvelope.data?.id}/attachments`, {
      data: {
        attachmentType: "OZ",
        attachmentControlNumber: `ATTACH-EXPORT-${Date.now()}`,
        reportTypeCode: "B4",
        transmissionCode: "EL",
        contentType: "text/plain",
        description: "Embedded notes for export",
        content: "Embedded 275 scroll content for document-vault testing."
      }
    });
    expect(attachment.status()).toBe(201);

    const attachmentEnvelope = (await attachment.json()) as Envelope;
    expect(attachmentEnvelope.transaction?.type).toBe("275");

    const reference = await request.get(`${serviceUrls.apiGateway}/v1/transactions/${attachmentEnvelope.transaction?.id}/document-reference`);
    expect(reference.ok()).toBeTruthy();
    const referenceEnvelope = (await reference.json()) as Envelope<DocumentReference>;
    expect(referenceEnvelope.data).toMatchObject({
      transactionId: attachmentEnvelope.transaction?.id,
      retrievalMode: "embedded",
      retrievalStatus: "available",
      embeddedContentAvailable: true
    });

    const content = await request.get(`${serviceUrls.apiGateway}/v1/transactions/${attachmentEnvelope.transaction?.id}/document-reference/content`);
    expect(content.ok()).toBeTruthy();
    expect(content.headers()["content-type"]).toContain("text/plain");
    await expect(await content.text()).toContain("Embedded 275 scroll content");
  });

  test("gateway exports and replays accepted XML intake audit messages", async ({ request }) => {
    const xmlName = uniqueDemoName("Replay Mage");
    const xml = `<?xml version="1.0" encoding="UTF-8"?>
<AshnX12Transaction type="834">
  <Sender id="partner-greenstone"/>
  <Receiver id="Adventure Society"/>
  <Enrollment>
    <Name>${xmlName}</Name>
    <Rank>Silver</Rank>
    <Guild>Replay Guild</Guild>
    <Region>Greenstone</Region>
  </Enrollment>
</AshnX12Transaction>`;

    const intake = await request.post(`${serviceUrls.apiGateway}/v1/x12/xml`, {
      headers: { "Content-Type": "application/xml" },
      data: xml
    });
    expect(intake.status()).toBe(201);

    const messages = await request.get(`${serviceUrls.apiGateway}/v1/x12/messages?limit=5&type=834&q=${encodeURIComponent(xmlName)}`);
    expect(messages.ok()).toBeTruthy();
    const messageEnvelope = (await messages.json()) as Envelope<InboundMessage[]>;
    const message = messageEnvelope.data?.find((item) => item.transactionType === "834" && item.status === "accepted");
    expect(message?.id).toBeTruthy();

    const xmlExport = await request.get(`${serviceUrls.apiGateway}/v1/x12/messages/${message?.id}/export`);
    expect(xmlExport.ok()).toBeTruthy();
    expect(xmlExport.headers()["content-type"]).toContain("application/xml");
    await expect(await xmlExport.text()).toContain(xmlName);

    const jsonExport = await request.get(`${serviceUrls.apiGateway}/v1/x12/messages/${message?.id}/export?format=json`);
    expect(jsonExport.ok()).toBeTruthy();
    const exportedMessage = (await jsonExport.json()) as InboundMessage;
    expect(exportedMessage.id).toBe(message?.id);
    expect(exportedMessage.transactionType).toBe("834");

    const replay = await request.post(`${serviceUrls.apiGateway}/v1/x12/messages/${message?.id}/replay`);
    expect(replay.status()).toBe(201);
    const replayEnvelope = (await replay.json()) as Envelope;
    expect(replayEnvelope.transaction?.type).toBe("834");
  });

  test("gateway accepts multipart batch intake with accepted and rejected scrolls", async ({ request }) => {
    const batchName = uniqueDemoName("Batch Bard");
    const acceptedXml = `<?xml version="1.0" encoding="UTF-8"?>
<AshnX12Transaction type="834">
  <Sender id="partner-greenstone"/>
  <Receiver id="Adventure Society"/>
  <Enrollment>
    <Name>${batchName}</Name>
    <Rank>Bronze</Rank>
    <Guild>Batch Guild</Guild>
    <Region>Greenstone</Region>
  </Enrollment>
</AshnX12Transaction>`;
    const rejectedXml = `<?xml version="1.0" encoding="UTF-8"?>
<AshnX12Transaction type="278">
  <Sender id="provider-vitesse-temple"/>
  <Receiver id="Adventure Society"/>
  <PriorAuthorization>
    <AdventurerId>adv-batch-missing</AdventurerId>
    <ProviderId>provider-vitesse-temple</ProviderId>
    <ServiceType>dragon-riding</ServiceType>
    <IncidentSeverity>Diamond</IncidentSeverity>
  </PriorAuthorization>
</AshnX12Transaction>`;

    const batch = await request.post(`${serviceUrls.apiGateway}/v1/x12/batch`, {
      multipart: {
        files: {
          name: "accepted-834.xml",
          mimeType: "application/xml",
          buffer: Buffer.from(acceptedXml)
        },
        file: {
          name: "rejected-278.xml",
          mimeType: "application/xml",
          buffer: Buffer.from(rejectedXml)
        }
      }
    });
    expect(batch.status()).toBe(207);
    const batchEnvelope = (await batch.json()) as Envelope<{ total: number; accepted: number; rejected: number; results: Array<{ accepted: boolean; statusCode: number; transactionType?: string; error?: string }> }>;
    expect(batchEnvelope.data).toMatchObject({ total: 2, accepted: 1, rejected: 1 });
    expect(batchEnvelope.data?.results.some((result) => result.accepted && result.transactionType === "834")).toBeTruthy();
    expect(batchEnvelope.data?.results.some((result) => !result.accepted && result.statusCode === 400)).toBeTruthy();
  });

  test("gateway manages trading partner profiles through the public route", async ({ request }) => {
    const senderId = `provider-e2e-${Date.now()}`;
    const partner = {
      name: "E2E Crystal Tower Partner",
      senderId,
      receiverId: "Adventure Society",
      allowedTransactionTypes: ["270", "275", "837"],
      routeTarget: "payer-core",
      status: "active"
    };

    const created = await request.post(`${serviceUrls.apiGateway}/v1/x12/trading-partners`, { data: partner });
    expect(created.status()).toBe(201);
    const createdEnvelope = (await created.json()) as Envelope<TradingPartner>;
    expect(createdEnvelope.data?.id).toBe(`tp-${senderId}`);
    expect(createdEnvelope.data?.allowedTransactionTypes).toEqual(["270", "275", "837"]);

    const listed = await request.get(`${serviceUrls.apiGateway}/v1/x12/trading-partners`);
    expect(listed.ok()).toBeTruthy();
    const listedEnvelope = (await listed.json()) as Envelope<TradingPartner[]>;
    expect(listedEnvelope.data?.some((item) => item.id === createdEnvelope.data?.id)).toBeTruthy();

    const updated = await request.put(`${serviceUrls.apiGateway}/v1/x12/trading-partners/${createdEnvelope.data?.id}`, {
      data: {
        ...partner,
        allowedTransactionTypes: ["270"],
        status: "inactive"
      }
    });
    expect(updated.ok()).toBeTruthy();
    const updatedEnvelope = (await updated.json()) as Envelope<TradingPartner>;
    expect(updatedEnvelope.data?.status).toBe("inactive");
    expect(updatedEnvelope.data?.allowedTransactionTypes).toEqual(["270"]);

    const deleted = await request.delete(`${serviceUrls.apiGateway}/v1/x12/trading-partners/${createdEnvelope.data?.id}`);
    expect(deleted.ok()).toBeTruthy();
  });

  test("gateway accepts dental 278 and 837D XML workflows", async ({ request }) => {
    const enrolled = await enrollAdventurer(request, "Dental Knight");

    const dentalAuthXml = `<?xml version="1.0" encoding="UTF-8"?>
<AshnX12Transaction type="278">
  <Sender id="provider-vitesse-temple"/>
  <Receiver id="Adventure Society"/>
  <PriorAuthorization>
    <AdventurerId>${enrolled.data?.id}</AdventurerId>
    <ProviderId>provider-vitesse-temple</ProviderId>
    <ServiceType>dental-predetermination</ServiceType>
    <IncidentSeverity>Normal</IncidentSeverity>
    <DentalService>
      <CDTCode>D7240</CDTCode>
      <ToothNumber>14</ToothNumber>
      <Surface>MO</Surface>
      <Quadrant>UR</Quadrant>
      <Orthodontic>false</Orthodontic>
    </DentalService>
  </PriorAuthorization>
</AshnX12Transaction>`;

    const auth = await request.post(`${serviceUrls.apiGateway}/v1/x12/xml`, {
      headers: { "Content-Type": "application/xml" },
      data: dentalAuthXml
    });
    expect(auth.status()).toBe(202);
    const authEnvelope = (await auth.json()) as Envelope;
    expect(authEnvelope.transaction?.type).toBe("278");
    expect(authEnvelope.transaction?.status).toBe("Pending");

    const dentalClaimXml = `<?xml version="1.0" encoding="UTF-8"?>
<AshnX12Transaction type="837D">
  <Sender id="provider-vitesse-temple"/>
  <Receiver id="Adventure Society"/>
  <Claim>
    <AdventurerId>${enrolled.data?.id}</AdventurerId>
    <ProviderId>provider-vitesse-temple</ProviderId>
    <IncidentSeverity>Normal</IncidentSeverity>
    <AmountCents>85000</AmountCents>
    <Diagnosis qualifier="ABK" primary="true"><Code>K021</Code></Diagnosis>
    <ServiceLine lineNumber="1">
      <ProcedureCode>D7240</ProcedureCode>
      <CDTCode>D7240</CDTCode>
      <Description>Removal of impacted tooth</Description>
      <Units>1</Units>
      <AmountCents>85000</AmountCents>
      <ToothNumber>14</ToothNumber>
      <Surface>MO</Surface>
      <Quadrant>UR</Quadrant>
      <Orthodontic>false</Orthodontic>
    </ServiceLine>
  </Claim>
</AshnX12Transaction>`;

    const claim = await request.post(`${serviceUrls.apiGateway}/v1/x12/xml`, {
      headers: { "Content-Type": "application/xml" },
      data: dentalClaimXml
    });
    expect(claim.status()).toBe(201);
    const claimEnvelope = (await claim.json()) as Envelope<{ id: string }>;
    expect(claimEnvelope.transaction?.type).toBe("837D");
    expect(claimEnvelope.transactions?.map((transaction) => transaction.type)).toEqual(["837D", "277CA"]);

    const claimDetail = await request.get(`${serviceUrls.apiGateway}/v1/claims/${claimEnvelope.data?.id}`);
    expect(claimDetail.ok()).toBeTruthy();
    const claimDetailEnvelope = (await claimDetail.json()) as Envelope<{
      serviceLines?: Array<{ procedureCode: string; cdtCode?: string; toothNumber?: string; surface?: string; quadrant?: string }>;
    }>;
    expect(claimDetailEnvelope.data?.serviceLines?.[0]).toMatchObject({
      procedureCode: "D7240",
      cdtCode: "D7240",
      toothNumber: "14",
      surface: "MO",
      quadrant: "UR"
    });
  });
});
