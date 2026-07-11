# edi-intake

XML/JSON/raw X12 intake service for ASHN X12-inspired transactions.

It exposes:

- `GET /health`
- `POST /x12/transactions`
- `POST /x12/xml`
- `POST /x12/raw`

The service accepts canonical ASHN transaction submissions as XML or JSON, plus first-pass delimiter-based raw X12 for `837` claims and `275` attachments. Use `POST /x12/transactions` for content-negotiated canonical intake, `POST /x12/xml` as the XML compatibility route, and `POST /x12/raw` for raw segment intake.

Supported request media types:

- `application/xml`, `text/xml`, and `*+xml`
- `application/json` and `*+json`
- `application/edi-x12`, `application/x12`, and `text/plain`

The service validates the canonical ASHN transaction envelope or parses the supported raw X12 subset, maps it into existing domain requests, and forwards accepted work to existing `payer-core` HTTP endpoints.

When `DATABASE_URL` is configured, every intake submission is written to `inbound_messages` with its raw payload, transaction type, accepted/rejected status, downstream response status, and validation error if present.

Architecture decisions:

- `api-gateway` exposes the public XML route at `POST /v1/x12/xml`; `edi-intake` stays behind the gateway.
- `api-gateway` also exposes `POST /v1/x12/transactions` for Rails-style content negotiation across XML and JSON representations.
- `edi-intake` translates and audits payloads but does not write payer transactions directly.
- `payer-core` remains the source of truth for business rules, ledger writes, claims, authorizations, payments, and async jobs.
- Canonical ASHN XML comes first; partner-specific or transaction-specific XML schemas can be added later.
- Rejected XML/JSON/raw X12 is still audited so failed submissions are visible, exportable, and replayable for demos/debugging.

Example:

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

Equivalent canonical JSON:

```json
{
  "type": "837",
  "sender": { "id": "provider-vitesse-temple" },
  "receiver": { "id": "Adventure Society" },
  "claim": {
    "adventurerId": "adventurer-id",
    "providerId": "provider-vitesse-temple",
    "incidentSeverity": "Awakened",
    "amountCents": "125000"
  }
}
```
