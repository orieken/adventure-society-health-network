# ASHN Elevator Pitch Presentation

Short-form talk track for a 3–5 minute demo introduction.

---

## Slide 1 — ASHN

**Adventure Society Health Network**

A fantasy-themed healthcare EDI simulator that makes X12 workflows visible, memorable, and demoable.

**Talk track:**  
ASHN turns the invisible world of healthcare EDI into a working story: adventurers, healers, claims, authorizations, acknowledgments, and payments.

---

## Slide 2 — The Problem

Healthcare EDI is essential, but hard to explain.

- The transaction flow is invisible.
- The language is dense.
- The systems are distributed.
- The learning curve is steep.

**Talk track:**  
Most people encounter X12 as a wall of acronyms: `834`, `270`, `837`, `835`. ASHN gives those acronyms a narrative and a working system.

---

## Slide 3 — The Metaphor

ASHN maps healthcare operations into a story world:

- **Adventure Society** = payer / health plan
- **Adventurer** = member / patient
- **Temple, clinic, outpost** = provider
- **Dungeon injury** = medical event
- **Ledger** = transaction history

**Talk track:**  
Instead of “a provider submits an 837 to a payer,” we say: a healer treats an injured adventurer and submits a claim to the Society. Same architecture, much easier to remember.

---

## Slide 4 — The Demo Flow

The complete lifecycle is visible end to end:

1. `834` enrollment
2. `270 → 271` eligibility
3. `278` prior authorization
4. `837 → 277CA` claim submission and acknowledgment
5. `276 → 277` claim status
6. `835` payment/remittance

**Talk track:**  
Every step becomes a persisted transaction. We can search it, filter it, inspect JSON, inspect raw X12, and see how each message relates to the business workflow.

---

## Slide 5 — What We Built

ASHN is a working local system:

- Go services for gateway, payer, provider, and EDI intake
- Postgres-backed transaction ledger and inbound XML audit
- XML intake that maps external submissions into payer workflows
- Raw X12 generation, display, copy, and download
- Dashboard filtering, pagination, and detail views
- Unit and DB-backed integration tests

**Talk track:**  
This is not just a diagram. It runs locally, persists state, accepts XML, emits acknowledgments, and gives us a dashboard we can use in demos or technical walkthroughs.

---

## Slide 6 — Why It Matters

ASHN helps teams understand EDI faster.

- **Product teams** see the business lifecycle.
- **Engineers** see service boundaries and data flow.
- **Analysts** see how X12 maps to real operations.
- **Stakeholders** get a memorable demo instead of abstract plumbing.

**Talk track:**  
The point is not to replace a clearinghouse. The point is to make the transaction model concrete before we go deeper into production-grade EDI.

---

## Slide 7 — The One-Minute Demo Script

“We enroll an adventurer into the Adventure Society with an `834`.

A healer checks coverage using `270 → 271`.

Because the injury is severe, they request authorization with a `278`.

After treatment, they submit an `837` claim. The Society returns a `277CA` acknowledgment.

The healer checks status with `276 → 277`, and the Society pays through an `835`.

The dashboard shows every event in the ledger, including 275 documentation packets, raw X12, replay tools, and XML intake audit records.”

---

## Slide 8 — Closing

**ASHN makes healthcare EDI tangible.**

It combines a memorable metaphor, a real service architecture, and a searchable transaction ledger so teams can see how payer, provider, claim, acknowledgment, and payment workflows fit together.

**Talk track:**  
Our next step is to harden the integration lab: API auth, correlation IDs, structured logs, traces, migration safety, rate limits, and deeper document-vault behavior for external 275 references.
