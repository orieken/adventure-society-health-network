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
    relatedId?: string;
  };
  transactions?: Array<{
    id: string;
    type: string;
    status: string;
    relatedId?: string;
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
        documentReferenceUrl: "https://docs.example.test/ashn/auth-notes.pdf"
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
      "HI*ABK:S062X9A~",
      "SV1*HC:ASHN1*1250.00*UN*1***1~",
      `SE*12*${rawControl}~`,
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

    const ledger = await request.get(`${serviceUrls.apiGateway}/v1/transactions?limit=5&type=837`);
    expect(ledger.ok()).toBeTruthy();

    const ledgerEnvelope = (await ledger.json()) as Envelope<Array<{ id: string; type: string }>>;
    expect(ledgerEnvelope.data?.some((transaction) => transaction.type === "837")).toBeTruthy();

    const attachmentLedger = await request.get(`${serviceUrls.apiGateway}/v1/transactions?limit=5&type=275`);
    expect(attachmentLedger.ok()).toBeTruthy();

    const attachmentLedgerEnvelope = (await attachmentLedger.json()) as Envelope<Array<{ id: string; type: string; relatedId?: string }>>;
    expect(attachmentLedgerEnvelope.data?.some((transaction) => transaction.type === "275")).toBeTruthy();
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
});
