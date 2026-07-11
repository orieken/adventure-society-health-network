# ASHN Elevator Pitch

ASHN — the Adventure Society Health Network — is a fantasy-themed healthcare EDI simulator that makes the X12 transaction lifecycle visible, memorable, and demoable.

Instead of starting with abstract payer/provider jargon, ASHN maps the real healthcare flow onto a story world: the Adventure Society is the payer, adventurers are covered members, and clinics or temples are providers. A dungeon injury becomes a claim. A resurrection request becomes prior authorization. A remittance becomes the Society paying the healer.

## The Story In One Minute

An adventurer joins the Society, gets coverage, visits a healer after a dangerous encounter, and the entire healthcare transaction chain unfolds:

1. **Enrollment** — the adventurer registers with the Society, creating an `834`.
2. **Eligibility** — the provider checks active coverage with `270 → 271`.
3. **Prior Authorization** — a high-severity treatment like resurrection sends a `278`.
4. **Claim Submission** — the provider submits the incident claim as an `837`.
5. **Claim Status** — the provider checks progress with `276 → 277`.
6. **Payment** — the Society sends remittance advice with an `835`.

The dashboard shows each step as both a lore-flavored event and a real EDI-inspired transaction record.

The current build also exposes XML/JSON intake, trading partner validation, raw X12 payloads, acknowledgments, asynchronous processing, export/replay controls, and 275 documentation review flows so learners can see both the business story and the integration mechanics.

For a deeper breakdown of how each X12 transaction fits into the project, see [ASHN X12 Workflow Breakdown](x12-workflow.md).

## Why It Matters

Healthcare EDI is important but hard to learn because the flow is invisible and the language is dense. ASHN turns that flow into a working system with:

- a Go API gateway and service architecture
- payer and provider service boundaries
- Postgres-backed transaction history
- XML/JSON intake with raw X12 visibility
- trading partner routing and validation
- asynchronous authorization and adjudication state changes
- 275 documentation requests, per-document review, deficiency follow-up, and resubmission
- a CLI and dashboard for demos
- repeatable local scripts for setup, reset, and workflow execution
- tests around the core HTTP contracts

It is not trying to be a production clearinghouse. It is a teaching, prototyping, and architecture playground for understanding how healthcare transactions move through a system.

## Demo Narrative

“We start by enrolling Farros, an Iron-rank adventurer, into the Adventure Society Health Network. That creates the equivalent of an X12 `834` enrollment transaction.

Then Farros arrives at the Temple of the Healer in Vitesse. Before treatment, the temple checks whether coverage is active. That is our `270` eligibility inquiry and `271` eligibility response.

Because the incident is severe, the temple requests prior authorization for resurrection using a `278`. If the Society needs supporting records, the provider can submit a `275` attachment packet and respond to document-specific deficiency requests.

After treatment, the temple submits an incident claim with an `837`. The provider can check the claim’s current status with `276 → 277`. Finally, the Society pays the claim and sends an `835` remittance advice.

Every one of those events is persisted in Postgres and visible in the dashboard ledger, so the system remembers what happened even after refresh or restart.”

## Closing Line

ASHN makes healthcare EDI tangible: a real transaction workflow, a memorable fantasy metaphor, and a working local system that demonstrates how payer, provider, claim, authorization, attachment, payment, audit, and ledger concepts fit together.
