# 275 Companion Guide Notes

This note captures the implementation-relevant ideas extracted from the local UHC and esMD 275 companion-guide PDFs so the project does not need to retain those large reference files in git.

## Source Guides Reviewed

- UHC 275 companion guide for X12 `006020X314`.
- esMD X12N 275 companion guide for additional information supporting a health care claim or encounter.

The project should treat this page as an implementation digest, not as a replacement for certification-grade TR3 or payer companion-guide review.

## Current ASHN Coverage

ASHN already implements a teaching-focused 275 workflow:

- Claim-linked and prior-auth-linked `275` attachments.
- Solicited claim documentation requests using `277` checklist metadata.
- Multi-document packets with `packetId`, `packetSequence`, and `packetCount`.
- Per-document business review statuses separate from EDI acceptance.
- Deficiency request and single-document resubmission flow.
- External document references and embedded-content download support.
- Raw X12-inspired correlation through `REF*1K`, `REF*G1`, `REF*6R`, `PWK`, `LQ`, `K3`, and `BIN`.
- Partner-specific validation for attachment type, report type, content type, control-number prefix, and embedded size.

## Extracted Companion-Guide Concepts

### Envelope and Transaction Shape

- Version target is `006020X314` for claim/encounter attachment support.
- A 275 interchange should remain transaction-set specific; do not mix unrelated transaction sets in the same file.
- Companion-guide examples emphasize `ISA`, `GS`, `ST`, `BGN`, party loops, `LX`, `TRN`, `DTP`, `CAT`, `OOI`, `BDS`, `SE`, `GE`, and `IEA`.
- `BGN01` differentiates attachment purpose: unsolicited attachment versus solicited attachment.
- `BGN02` should be unique enough to trace the transaction set.

### Solicited vs Unsolicited Attachments

- Unsolicited attachments are sent with or near the original claim and need to correlate back to the 837/PWK control values.
- Solicited attachments respond to a payer request and need trace matching against the payer request trace.
- For solicited flows, ASHN should preserve a payer-generated request trace and require the response `275` to echo it.
- For unsolicited flows, ASHN should preserve the originating claim attachment control number and use it to relate the `837`, `275`, and eventual response.

### Attachment Loops and Limits

- Attachment payloads are modeled as repeated attachment loops; a practical companion-guide cap is 10 `LX` loops per transaction set.
- If more than 10 documents are needed, split the submission across multiple 275 transaction sets.
- A transaction-set-level total attachment size limit should be configurable. UHC-style guidance references a 100 MB cumulative limit per `ST`/`SE`, but ASHN can keep lower demo defaults.
- Attachment control numbers should not be duplicated within the same claim/request context.

### MIME and Binary Data

- Companion guides distinguish ASCII and Base64 attachment encoding through `BDS01` values such as `ASC` and `B64`.
- Attachment content should be packaged as a single-part MIME payload when embedded.
- The MIME type, file extension, and declared content type should agree.
- Base64 validation should detect corrupt or non-decodable content.
- Future-facing implementations should expect Base64 to become the safer default for binary documents.

### File and Document Validation

- Partner profiles should define accepted file extensions and MIME types.
- Common attachment document types include text, PDF, image, and TIFF-style artifacts; ASHN should model these as profile data instead of hard-coding one global list.
- Rejections should distinguish unsupported file extension, MIME mismatch, invalid encoding, oversized packet, missing correlation, and invalid purpose indicators.

### Timing Rules

- Unsolicited claim attachments should be submitted close to the originating claim.
- A guide-inspired rule is to send the claim and unsolicited attachment the same day, with a configurable late window such as five calendar days.
- Late attachments should produce a clear intake rejection and suggest resubmitting the claim/attachment pair or responding through the solicited path when appropriate.

### Acknowledgments and Rejections

- Syntax/interchange problems belong in `TA1` or `999` style acknowledgments.
- Application/business attachment validation failures can be represented through `824` application reporting.
- ASHN should keep these distinct from clinical/business document review outcomes such as `Received`, `Accepted`, or `Rejected`.
- Useful rejection scenarios to model:
  - invalid `BGN01` purpose indicator
  - invalid `CAT02` attachment format indicator
  - unsupported file extension or MIME type
  - corrupt Base64/MIME payload
  - too many attachment loops
  - duplicate attachment control number
  - missing payer request trace for solicited 275
  - claim not found or claim already released/rejected
  - late unsolicited attachment

## Implementation TODOs

- [x] Add `attachmentPurpose` to 275 payloads: `unsolicited` and `solicited`.
- [x] Generate `BGN` for raw 275 output and parse `BGN01/BGN02` from inbound raw 275.
- [x] Store payer request trace for solicited claim documentation requests and require response trace matching.
- [x] Store originating 837/PWK attachment control values for unsolicited claim attachments.
- [x] Add a `275` raw shape closer to `006020X314` with party loops, `LX`, `TRN`, `DTP`, `CAT`, `OOI`, and `BDS`.
- [x] Add `BDS01` support for `ASC`, `B64`, and `REF`, including Base64 decode validation.
- [x] Add partner-configurable file-extension allowlists.
- [x] Validate declared content type against attachment file extension and reject multipart Base64 packaging.
- [x] Add full single-part MIME packaging validation.
- [x] Add configurable max `LX` loop count and transaction-set attachment byte limits.
- [x] Add duplicate attachment-control-number detection by claim/request context.
- [x] Add same-day and late-window validation for unsolicited 275 claim attachments.
- [x] Emit or simulate `824` application reporting for attachment validation failures.
- [x] Add `TA1` pre-screen outcomes for envelope/interchange rejection examples.
- [x] Add dashboard drilldowns that separate `TA1`, `999`, `824`, and business review statuses.
- [x] Add demo fixtures for invalid `BGN01`, invalid `CAT02`, oversized packet, corrupt Base64, missing trace, and late attachment.

## Design Guardrails

- Keep ASHN's canonical JSON/XML intake simple; model companion-guide detail as optional profile-driven metadata.
- Do not turn `payer-core` into an X12 parser. Keep parsing, partner validation, and acknowledgment concerns in `edi-intake`.
- Keep transaction acceptance separate from document sufficiency review.
- Prefer small teachable fixtures over full clearinghouse certification behavior unless the project explicitly pivots to certification-grade validation.
