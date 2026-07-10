# ASHN Future Enhancements TODO

This backlog captures the next useful build paths for ASHN after the current JSON-backed X12 simulation. The north star is to keep the project demoable while gradually moving closer to real healthcare EDI patterns.

## Recommended Next Milestone

Build an **EDI intake service** that accepts XML payloads, validates them, converts them into ASHN's internal transaction model, audits every submission, and forwards accepted work to `payer-core`.

This keeps `payer-core` focused on business state while giving us a clean place to experiment with external data formats.

## Priority Backlog

### P0 — XML EDI Intake Service

- [x] Add a new `apps/edi-intake` service.
- [x] Expose `POST /x12/xml` for XML transaction submissions.
- [x] Accept `Content-Type: application/xml` and `text/xml`.
- [x] Parse XML into a neutral inbound envelope.
- [x] Detect transaction type: `834`, `270`, `275`, `278`, `837`, `276`, `835`, `820`.
- [x] Validate required fields per transaction type.
- [x] Return structured validation errors for malformed or incomplete XML.
- [x] Convert accepted XML into ASHN payer requests.
- [x] Forward accepted work to `payer-core` through internal HTTP APIs.
- [x] Add gateway route `POST /v1/x12/xml`.
- [x] Add unit tests for valid XML, invalid XML, missing fields, and unsupported transaction types.
- [x] Persist raw inbound XML for audit/debug replay.
- [x] Add DB-backed integration tests through `api-gateway → edi-intake → payer-core`.

Suggested service boundary:

```mermaid
flowchart LR
    External["External Partner / Demo XML"] --> Gateway["api-gateway"]
    Gateway --> Intake["edi-intake"]
    Intake --> Validation["XML validation + mapping"]
    Intake --> Audit["inbound_messages audit"]
    Validation --> Payer["payer-core HTTP endpoints"]
    Payer --> Ledger["transaction ledger"]
```

Why this deserves a new service:

- XML and X12 parsing concerns are different from payer business logic.
- External payload validation should not clutter `payer-core`.
- It gives us a realistic integration boundary for partner submissions.
- It can later support raw X12, XML, JSON, file drops, and async queues.
- `payer-core` remains the source of truth for business rules, state transitions, transaction generation, and async jobs.

Important nuance: real X12 is often exchanged as delimiter-based EDI text rather than XML. Many enterprise systems also use XML wrappers, canonical XML, or XML-based integration contracts around EDI workflows. For ASHN, XML is a good next step because it is easier to inspect, validate, demo, and map into our domain model before we add raw X12 segment parsing.

## XML Intake Architecture Decisions

- **Routing:** Public submissions go through `api-gateway` at `POST /v1/x12/transactions` for content-negotiated XML/JSON intake. `POST /v1/x12/xml` remains as an XML compatibility route.
- **Business ownership:** `edi-intake` does not write payer transactions directly. It validates, maps, audits, and calls existing `payer-core` endpoints so one service owns business behavior.
- **Canonical contract:** Start with one canonical ASHN transaction envelope. XML uses `<AshnX12Transaction type="837">`; JSON uses the same shape with `type`, `sender`, `receiver`, and transaction-specific payload objects. Transaction-specific or partner-specific schemas can layer on later.
- **Audit policy:** Accepted and rejected XML submissions both create `inbound_messages` audit records. Rejections keep raw payload, error, transaction type when detectable, and downstream status when applicable.
- **Representation model:** Treat XML and JSON like Rails-style representations at the API edge: the gateway exposes one public workflow surface while intake services translate content types into canonical domain requests.

### P1 — Raw X12 Generation

- [x] Generate raw X12-like strings alongside the current JSON payloads.
- [x] Add envelope segments: `ISA`, `GS`, `ST`, `BHT`, `SE`, `GE`, `IEA`.
- [x] Add transaction-specific segment examples for `834`, `270`, `271`, `275`, `278`, `837`, `835`, `276`, and `277`.
- [x] Store raw X12 text on each ledger transaction.
- [x] Show raw X12 in the dashboard transaction detail panel.
- [x] Add copy buttons for raw transaction payloads.
- [x] Add download buttons for raw transaction payloads.
- [x] Expand segment generation toward companion-guide examples.
- [x] Add XML intake validation rules per transaction type.
- [x] Add full companion-guide validation profiles per trading partner.

### P1 — Acknowledgments

- [x] Add `999` implementation acknowledgment for accepted or rejected syntax.
- [x] Add `277CA` claim acknowledgment after `837` submission.
- [x] Track acknowledgment relationships between source transactions and responses.
- [x] Add dashboard filters for acknowledgment transaction types.
- [x] Add tests for accepted and rejected acknowledgment flows.

### P1 — Asynchronous Processing

- [x] Turn `apps/tx-worker` into an active worker service.
- [x] Add a transaction queue table or lightweight message queue.
- [x] Move long-running authorization and adjudication work off the request path.
- [x] Add retry, dead-letter, and replay behavior.
- [x] Show async status transitions in the dashboard.

### P2 — Prior Authorization Lifecycle

- [x] Add explicit `278` approval and denial endpoints.
- [x] Add authorization review state: `Pending`, `Approved`, `Denied`.
- [x] Add severity and service-type rules for auto-approval.
- [x] Link authorization decisions to downstream claims.
- [x] Show authorization history in claim detail views.

### P2 — Claim Adjudication

- [x] Add baseline adjudication rules based on severity and billed amount.
- [x] Calculate allowed amount, patient responsibility, paid amount, and denial reasons.
- [x] Add denial and partial-payment scenarios.
- [x] Expand `835` payloads with claim adjustment and remittance details.
- [x] Add tests for paid adjudication and remittance detail.
- [x] Add `275` patient information attachments linked to claim transactions.
- [x] Add payer-specific `275` companion-guide validation and timeline attachment labels.
- [x] Add solicited claim attachment requests that move claims into `Pending Documentation`.
- [x] Add a 275 Documentation Workbench for checklist requests and packet submission.
- [x] Allow `275` attachments to link to pending `278` prior authorization reviews.
- [x] Add attachment review outcomes distinct from transaction acceptance.
- [x] Support external document references for large PDFs/images instead of embedded `BIN` content.
- [x] Support multi-attachment packets grouped under a claim or authorization.
- [x] Move payer-specific `275` validation rules into trading partner profile data.
- [x] Add richer rules based on provider tier, adventurer rank, benefits, and coverage status.
- [x] Add more tests for denied and partially paid claim variants.

### P2 — Trading Partners and Routing

- [x] Add trading partner records.
- [x] Add sender/receiver identifiers distinct from internal IDs.
- [x] Add routing rules by transaction type and partner.
- [x] Add partner-specific validation profiles.
- [x] Add dashboard visibility for partner configuration.
- [x] Add create/update/delete partner management screens.
- [x] Add partner-specific companion-guide validation rules.

### P2 — Dashboard Enhancements

- [x] Add a transaction timeline view grouped by adventurer or claim.
- [x] Add saved filters for transaction type, status, provider, and date range.
- [x] Add raw payload tabs: JSON, XML, and X12.
- [x] Add XML intake audit visibility with raw XML detail.
- [x] Add transaction export to JSON, XML, and X12.
- [x] Add XML intake audit export to XML and JSON.
- [x] Add replay controls for transactions and inbound XML messages.
- [x] Add ledger export to CSV.
- [x] Add visual links between request/response transaction pairs.

### P3 — Security and Operational Readiness

- [ ] Add API authentication for partner-facing endpoints.
- [ ] Add request IDs and correlation IDs across services.
- [ ] Add structured logs.
- [ ] Add basic OpenTelemetry traces.
- [x] Add health checks for every service in Docker Compose.
- [ ] Add migration tests and seed-data reset tests.
- [ ] Add rate limiting for public/demo endpoints.

## Proposed XML Shape

The first XML contract should be intentionally simple and canonical. It does not need to mirror every real X12 segment on day one.

Example `837` claim submission:

```xml
<AshnX12Transaction type="837">
  <Sender id="provider-vitesse-temple" />
  <Receiver id="Adventure Society" />
  <Claim>
    <AdventurerId>adventurer-id</AdventurerId>
    <ProviderId>provider-vitesse-temple</ProviderId>
    <IncidentSeverity>Awakened</IncidentSeverity>
    <AmountCents>125000</AmountCents>
  </Claim>
</AshnX12Transaction>
```

Example `270` eligibility inquiry:

```xml
<AshnX12Transaction type="270">
  <Sender id="provider-vitesse-temple" />
  <Receiver id="Adventure Society" />
  <EligibilityInquiry>
    <AdventurerId>adventurer-id</AdventurerId>
    <ProviderId>provider-vitesse-temple</ProviderId>
  </EligibilityInquiry>
</AshnX12Transaction>
```

## Proposed Implementation Order

1. Create `apps/edi-intake` with health check and XML parsing.
2. Add domain structs for inbound XML envelopes.
3. Add tests for XML decoding and validation.
4. Add gateway route: `POST /v1/x12/xml`.
5. Forward accepted XML requests into existing `payer-core` endpoints.
6. Persist raw XML alongside generated transaction records.
7. Add dashboard XML payload display.
8. Add raw X12 generation after XML intake is stable.

## Decision Summary

Start with canonical ASHN XML through the public gateway and a dedicated `edi-intake` service. Keep the XML contract small, strongly validated, fully audited, and easy to demo. Forward accepted work into existing `payer-core` endpoints instead of bypassing business rules. Once that works, add raw X12 segment parsing, partner-specific XML variants, and richer content negotiation.

That path gets us closer to real enterprise EDI without burying the project in full X12 implementation complexity too early.
