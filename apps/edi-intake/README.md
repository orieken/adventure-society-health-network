# edi-intake

XML intake service for ASHN X12-inspired transactions.

It exposes:

- `GET /health`
- `POST /x12/xml`

The service accepts `application/xml` or `text/xml`, validates the canonical ASHN XML envelope, maps it into existing domain requests, and forwards accepted work to `payer-core`.

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
