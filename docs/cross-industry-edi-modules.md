# Cross-Industry EDI Module Notes

ASHN should keep `101` and `110` out of the healthcare payer/provider workflow. They are useful learning targets, but they belong in separate modules with their own domain stories, partner profiles, validation rules, raw samples, and dashboard views.

This keeps the core ASHN narrative clean: healthcare transactions teach enrollment, eligibility, authorization, claims, attachments, remittance, audit, and partner validation. Cross-industry EDI can become a nearby lab without muddying those workflows.

## `101` Name and Address Lists

`101` is a general-business transaction set for exchanging name and address lists. It is not a healthcare claim, eligibility, authorization, payment, or attachment transaction.

If ASHN explores `101`, model it as a **directory synchronization module** rather than a payer workflow.

Suggested module shape:

- **Service boundary:** `apps/directory-intake` or an isolated route group under a future EDI lab service.
- **Domain model:** organizations, locations, contacts, addresses, external identifiers, effective dates, and list purpose.
- **Partners:** sender/receiver profiles for supplier directories, employer rosters, facility lists, or vendor address books.
- **Validation:** required party identifiers, address line completeness, country/state normalization, duplicate external IDs, and effective-date windows.
- **Raw samples:** small `101` fixtures for new list, replacement list, and correction/update list.
- **Dashboard view:** directory import runs with accepted/rejected counts, changed records, and drilldown by party/location.

Good ASHN learning outcome: show that X12 can carry master-data style lists, but keep that separate from healthcare transaction state.

## `110` Air Freight Details and Invoice

`110` is a transportation transaction set for air freight details and invoices. It belongs to logistics and billing, not healthcare payer/provider administration.

If ASHN explores `110`, model it as a **transportation invoice module**.

Suggested module shape:

- **Service boundary:** `apps/logistics-intake` or an isolated route group under a future EDI lab service.
- **Domain model:** shipment, air waybill, origin/destination, carrier, consignee, line charges, accessorial charges, taxes, and invoice status.
- **Partners:** carrier, shipper, consignee, freight-audit vendor, and accounts-payable receiver profiles.
- **Validation:** invoice number uniqueness, carrier identifiers, shipment reference matching, charge totals, currency, date windows, and route/location codes.
- **Raw samples:** small `110` fixtures for invoice accepted, invoice rejected for total mismatch, and duplicate invoice.
- **Dashboard view:** freight invoices with charge breakdown, route lane, partner rejection reasons, and replay/export controls.

Good ASHN learning outcome: contrast healthcare remittance/adjudication with logistics invoicing while reusing generic EDI concepts such as envelope parsing, partner validation, acknowledgments, and audit.

## Isolation Rules for a Broader EDI Lab

If ASHN becomes a broader EDI lab, non-healthcare modules should follow these boundaries:

- **Separate navigation:** add a top-level “EDI Lab” or module picker instead of mixing `101`/`110` into the healthcare dashboard cards.
- **Separate routes:** expose module-specific endpoints such as `/v1/edi-lab/directories` or `/v1/edi-lab/freight-invoices`.
- **Separate domain tables:** avoid forcing directory records or freight invoices into claim/auth/payment tables.
- **Shared infrastructure only:** reuse audit, raw payload storage, replay, export, OpenAPI docs, and parser adapter interfaces.
- **Separate partner profiles:** keep healthcare companion-guide profiles distinct from supply-chain or transportation profile rules.
- **Separate fixtures:** store raw samples under module-specific fixture folders so tests explain the transaction's business context.
- **Separate dashboard widgets:** show purpose-built views rather than overloading the healthcare ledger with unrelated statuses.

## Recommended Decision

For the current project, keep `101` and `110` documented but unimplemented. The healthcare workflow is already rich enough for the main demo, and the next engineering work should favor parser boundaries, partner variations, and operational observability inside the healthcare scope.

If the project later expands into a general EDI teaching lab, start with one isolated proof-of-concept module:

1. Add a separate module landing card.
2. Create one domain table.
3. Add one canonical XML/JSON shape.
4. Add one raw X12 fixture.
5. Add partner validation and audit.
6. Add one dashboard drilldown.

That gives the broader lab a clean spine without turning ASHN’s healthcare story into soup. Tasty soup, maybe, but still soup.
