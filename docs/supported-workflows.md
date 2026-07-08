# ASHN Supported Workflows

This guide shows the workflows currently supported by Adventure Society Health Network. It is meant for demos, onboarding, and roadmap planning. The deeper transaction breakdown lives in [`x12-workflow.md`](x12-workflow.md).

## System Context

```mermaid
flowchart LR
    Dashboard["Dashboard / Demo Operator"] --> Gateway["api-gateway"]
    CLI["ashn-cli"] --> Gateway
    Partner["Trading Partner XML"] --> Gateway

    Gateway --> Payer["payer-core"]
    Gateway --> Intake["edi-intake"]
    Gateway --> Provider["provider-service"]

    Intake --> Payer
    Payer --> Ledger[("Postgres Ledger")]
    Intake --> Audit[("Inbound XML Audit")]
    Worker["tx-worker"] --> Ledger

    Ledger --> Dashboard
    Audit --> Dashboard
```

ASHN supports both **business-state APIs** and an **EDI-style transaction ledger**. A claim, authorization, or adventurer has current state, while every X12-inspired event is also persisted as a transaction record.

## Workflow Coverage Matrix

| Workflow | X12 transactions | Current entry points | Current UI support | Notes |
| --- | --- | --- | --- | --- |
| Enrollment | `834` | `POST /v1/adventurers`, XML `834` | Workflow card, ledger, timeline | Creates adventurer and enrollment transaction. |
| Eligibility | `270 → 271` | `POST /v1/eligibility`, XML `270` | Workflow card, ledger, timeline | Returns active/inactive coverage. |
| Prior authorization | `278` | `POST /v1/auth-requests`, `POST /v1/auth-requests/{id}/decision`, XML `278` | Workflow card, manual review widget, ledger, timeline | Starts pending; manual approve/deny or async worker decision. |
| Claim submission | `837 → 277CA` | `POST /v1/claims`, XML `837` | Workflow card, claims panel, ledger, timeline | Emits claim and claim acknowledgment. |
| Claim attachment | `277 → 275` | `POST /v1/claims/{id}/documentation-request`, `POST /v1/claims/{id}/attachments`, XML `275` | Claim detail action, ledger, timeline attachment label, raw X12 detail | Payer can request documentation; 275 clears the hold. |
| Claim status | `276 → 277` | `GET /v1/claims/{id}/status`, XML `276` | Ledger, timeline | Creates request/response status pair. |
| Payment/remittance | `835` | `POST /v1/claims/{id}/payment`, XML `835` | Workflow card, claims panel, ledger, detail drawer | Includes allowed, paid, adjustment, denial fields. |
| XML intake audit | `999` plus routed transaction | `POST /v1/x12/xml` | XML Intake tab, export/replay | Accepted/rejected XML submissions create audit records and acknowledgments. |
| Export/replay | JSON/XML/X12 exports | `/export`, `/replay` endpoints | Detail drawer buttons | Supports demo reset, replay, and artifact inspection. |

## 1. Enrollment Lifecycle

```mermaid
sequenceDiagram
    participant Demo as Dashboard / CLI
    participant Gateway as api-gateway
    participant Payer as payer-core
    participant Ledger as Transaction Ledger

    Demo->>Gateway: POST /v1/adventurers
    Gateway->>Payer: POST /enrollments
    Payer->>Payer: Create adventurer record
    Payer->>Ledger: Emit 834 Accepted
    Payer-->>Gateway: Adventurer + 834 transaction
    Gateway-->>Demo: Enrollment result
```

**What to show in a demo**

- Adventurer appears in the dashboard.
- Ledger contains an `834` transaction.
- Raw X12 detail includes enrollment-style segments.

## 2. Eligibility Lifecycle

```mermaid
sequenceDiagram
    participant Provider as Provider / Dashboard
    participant Gateway as api-gateway
    participant Payer as payer-core
    participant Ledger as Transaction Ledger

    Provider->>Gateway: POST /v1/eligibility
    Gateway->>Payer: POST /eligibility/query
    Payer->>Ledger: Emit 270 Dispatched
    Payer->>Payer: Check adventurer coverage
    alt Active coverage
        Payer->>Ledger: Emit 271 Accepted
    else Inactive coverage
        Payer->>Ledger: Emit 271 Denied
    end
    Payer-->>Gateway: Eligibility envelope
    Gateway-->>Provider: 270 + 271 result
```

**Current behavior**

- `270` represents the inquiry.
- `271` represents the payer response.
- The response is based on the adventurer coverage status in payer-core.

## 3. Prior Authorization Lifecycle

```mermaid
stateDiagram-v2
    [*] --> Pending: 278 request created
    Pending --> Approved: Manual approve
    Pending --> Denied: Manual deny
    Pending --> Approved: tx-worker auto review
    Pending --> Denied: tx-worker auto review
    Approved --> [*]
    Denied --> [*]
```

```mermaid
sequenceDiagram
    participant Reviewer as Dashboard Reviewer
    participant Gateway as api-gateway
    participant Payer as payer-core
    participant Worker as tx-worker
    participant Ledger as Transaction Ledger

    Reviewer->>Gateway: POST /v1/auth-requests
    Gateway->>Payer: POST /auth-requests
    Payer->>Ledger: Emit 278 Pending
    Payer->>Worker: Queue auth_review job
    Payer-->>Reviewer: Pending 278

    alt Manual review
        Reviewer->>Gateway: POST /v1/auth-requests/{txId}/decision
        Gateway->>Payer: Approve or deny
        Payer->>Ledger: Update 278 to Approved/Denied
    else No manual review
        Worker->>Ledger: Process pending auth_review
        Worker->>Ledger: Update 278 to Approved/Denied
    end
```

**Current behavior**

- The dashboard shows a prior-auth review widget after a `278` is created.
- `Approve Auth` and `Deny Auth` update the visible transaction status.
- The worker skips already-reviewed authorizations so manual decisions are not overwritten.

## 4. Claim Submission and Acknowledgment

```mermaid
sequenceDiagram
    participant Provider as Provider / Dashboard
    participant Gateway as api-gateway
    participant Payer as payer-core
    participant Worker as tx-worker
    participant Ledger as Transaction Ledger

    Provider->>Gateway: POST /v1/claims
    Gateway->>Payer: POST /claims
    Payer->>Payer: Create claim record
    Payer->>Ledger: Emit 837 Accepted
    Payer->>Ledger: Emit 277CA Accepted
    Payer->>Worker: Queue claim_adjudication
    Payer-->>Provider: Claim + 837 + 277CA

    Worker->>Ledger: Mark claim Pending
    Worker->>Ledger: Finalize claim Approved/Denied
    Worker->>Ledger: Emit related 277 status update
```

**Current behavior**

- `837` is the claim submission.
- `277CA` acknowledges that the payer accepted the claim for processing.
- `tx-worker` later adjudicates the claim.

## 5. Claim Attachment Lifecycle

```mermaid
sequenceDiagram
    participant Provider as Provider / XML Partner
    participant Gateway as api-gateway
    participant Payer as payer-core
    participant Ledger as Transaction Ledger

    Provider->>Gateway: POST /v1/claims/{claimId}/attachments or XML 275
    Gateway->>Payer: POST /claims/{claimId}/attachments
    Payer->>Payer: Validate payer-specific metadata
    alt Metadata valid
        Payer->>Ledger: Emit 275 Accepted linked to claim/837
        Payer->>Payer: Clear Pending Documentation hold
        Payer-->>Provider: Attachment accepted
    else Metadata invalid
        Payer-->>Provider: 400 invalid attachment
    end
```

```mermaid
flowchart TD
    A["275 Attachment Request"] --> B{"Provider profile"}
    B --> C["Vitesse Temple rules"]
    B --> D["Rimaros Hospital rules"]
    C --> E{"OZ + B4 + EL + text/plain + 4 KB"}
    D --> F{"OZ/PN + 03/B4 + EL + text/plain/pdf + 8 KB"}
    E --> G["Emit 275 + raw X12"]
    F --> G
    G --> H["Timeline: OZ/B4 attachment"]
```

**Current behavior**

- `275` is currently claim-linked through `claimId` and `relatedId`.
- Payers can mark a claim `Pending Documentation` and emit a related `277` documentation request.
- A valid `275` clears the documentation hold back to `Pending` so adjudication can continue.
- Raw X12 includes `REF*1K`, `REF*6R`, `PWK`, `LQ*AT`, `K3`, and `BIN`.
- The timeline labels 275 steps using attachment/report metadata.

## 6. Claim Status Lifecycle

```mermaid
sequenceDiagram
    participant Provider as Provider / Dashboard
    participant Gateway as api-gateway
    participant Payer as payer-core
    participant Ledger as Transaction Ledger

    Provider->>Gateway: GET /v1/claims/{claimId}/status
    Gateway->>Payer: GET /claims/{claimId}/status
    Payer->>Ledger: Emit 276 Dispatched
    Payer->>Ledger: Emit 277 Accepted
    Payer-->>Provider: Current claim status
```

**Current behavior**

- `276` is the provider inquiry.
- `277` is the payer response.
- The dashboard timeline can group these with the related claim.

## 7. Payment and Remittance Lifecycle

```mermaid
sequenceDiagram
    participant Operator as Dashboard Operator
    participant Gateway as api-gateway
    participant Payer as payer-core
    participant Ledger as Transaction Ledger

    Operator->>Gateway: POST /v1/claims/{claimId}/payment
    Gateway->>Payer: POST /claims/{claimId}/payment
    Payer->>Payer: Apply remittance/adjudication amounts
    Payer->>Ledger: Emit 835 Paid
    Payer-->>Operator: Paid claim + remittance transaction
```

**Current behavior**

- `835` includes billed, allowed, paid, adjustment, and patient responsibility fields.
- Payment updates claim status to `Paid`.
- Raw X12 detail shows remittance-inspired segments.

## 8. XML Intake, Acknowledgment, Export, and Replay

```mermaid
sequenceDiagram
    participant Partner as Trading Partner
    participant Gateway as api-gateway
    participant Intake as edi-intake
    participant Payer as payer-core
    participant Audit as XML Audit Store
    participant Ledger as Transaction Ledger

    Partner->>Gateway: POST /v1/x12/xml
    Gateway->>Intake: POST /x12/xml
    Intake->>Audit: Persist raw XML
    Intake->>Intake: Validate XML + trading partner rules
    alt Accepted
        Intake->>Payer: Forward mapped request
        Payer->>Ledger: Emit routed transaction
        Intake->>Ledger: Record 999 Accepted
        Intake-->>Partner: 201 accepted
    else Rejected
        Intake->>Ledger: Record 999 Failed
        Intake-->>Partner: 4xx validation error
    end
```

```mermaid
flowchart LR
    Detail["Dashboard detail drawer"] --> ExportJSON["Export JSON"]
    Detail --> ExportXML["Export XML"]
    Detail --> ExportX12["Export X12"]
    Detail --> Replay["Replay transaction/XML"]
    Replay --> Ledger["New related ledger event"]
```

**Current behavior**

- XML intake supports canonical ASHN XML wrappers for multiple transaction types.
- Every inbound XML message is visible in the XML Intake tab.
- Transactions and XML messages can be exported and replayed for demos.

## Recommended 275 Workflows To Add Next

ASHN already supports claim-linked `275` attachments. The next high-value workflows are:

### 1. Solicited Claim Attachment Request

```mermaid
sequenceDiagram
    participant Payer as payer-core
    participant Provider as Provider
    participant Ledger as Transaction Ledger

    Payer->>Ledger: 277 requests additional documentation
    Provider->>Ledger: 275 sends requested attachment
    Payer->>Ledger: Claim adjudication resumes
```

Baseline support now exists: a claim can move to `Pending Documentation`, emit a `277`, and accept a `275` that clears the hold. The next iteration should make the request reason/code more structured and show the request as a first-class attachment task.

### 2. Prior Authorization Attachment

```mermaid
sequenceDiagram
    participant Provider as Provider
    participant Payer as payer-core
    participant Ledger as Transaction Ledger

    Provider->>Payer: 278 prior auth request
    Payer->>Ledger: 278 Pending
    Provider->>Payer: 275 medical necessity attachment linked to 278
    Payer->>Ledger: 278 Approved/Denied after review
```

Allow `275` to link to a `278` transaction, not just a claim/`837`. This would make resurrection medical necessity feel more realistic.

### 3. Attachment Review Outcomes

Track attachment review state separately from transaction acceptance:

```mermaid
stateDiagram-v2
    [*] --> Received
    Received --> InReview
    InReview --> Accepted
    InReview --> Rejected
    Rejected --> Resubmitted
    Resubmitted --> InReview
    Accepted --> [*]
```

A `275` can be syntactically accepted but clinically rejected as insufficient. That distinction is useful for teaching EDI vs business decisions.

### 4. External Document Reference Mode

Support attachments that reference external documents instead of embedding content in `BIN`:

```mermaid
flowchart LR
    Provider["Provider"] --> Metadata["275 metadata"]
    Metadata --> Pointer["URL / document ID / object key"]
    Pointer --> Repository["Document repository"]
    Metadata --> Payer["payer-core review"]
```

This would model common enterprise patterns where large PDFs/images are stored elsewhere and the EDI transaction carries metadata plus a retrieval pointer.

### 5. Multi-Attachment Bundles

Allow one claim or auth to receive multiple attachment documents:

- operative note
- discharge summary
- lab report
- itemized bill
- medical necessity letter

The dashboard could show a compact “attachment packet” timeline grouped under the claim or authorization.

### 6. Payer-Specific Attachment Matrix

Move the current hardcoded rules into trading partner/profile data:

- allowed attachment types
- allowed report type codes
- max content size
- accepted content types
- required control prefixes
- solicited vs unsolicited rules

This is the cleanest next architecture step if ASHN keeps leaning into companion-guide learning.
